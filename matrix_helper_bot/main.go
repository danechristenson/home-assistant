// Matrix Helper Bot: lets chosen people run a few admin tasks from chat.
//
// Commands only come from the bot's own encrypted "Server helpers" room,
// which only the people listed in the add-on options are in, and only from
// devices their owner has verified (cross-signed). The bot is a server
// admin, so it checks every request itself: who may do what comes from the
// options, and it only ever sends the admin commands in this file, with
// validated arguments.
//
// Resetting a password: the bot asks the server to generate one in the Admin
// Room (which can't be encrypted: the server's own admin bot has no
// encryption), deletes the server's reply as soon as it has read it, posts
// the password in the encrypted helpers room, and deletes that message after
// a few minutes. It refuses to touch server admins, helpers, deactivated
// accounts and the bot itself.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const (
	optionsFile  = "/data/options.json"
	stateFile    = "/data/state.json"
	pickleFile   = "/data/pickle.key"
	cryptoDBFile = "/data/bot.db"

	resetsPerHour      = 5
	membershipInterval = 10 * time.Minute
	// Devices the bot shares keys with and takes commands from: signed by
	// their owner's cross-signing key (what Element calls a verified session)
	minTrust = id.TrustStateCrossSignedUntrusted
)

var localpartPattern = regexp.MustCompile(`^[a-z0-9._=/+-]+$`)

type Options struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	MessageMinutes int    `json:"password_message_minutes"`
	MatrixURL      string `json:"matrix_url"`
	Helpers        []struct {
		User           string `json:"user"`
		ResetPasswords bool   `json:"reset_passwords"`
	} `json:"helpers"`
}

type Permissions struct {
	ResetPasswords bool
}

type Redaction struct {
	Room  id.RoomID  `json:"room"`
	Event id.EventID `json:"event"`
	Due   time.Time  `json:"due"`
}

// State survives restarts
type State struct {
	HelpersRoom       id.RoomID                 `json:"helpers_room"`
	CrossSignedDevice id.DeviceID               `json:"cross_signed_device"`
	Redactions        []Redaction               `json:"redactions"`
	Resets            map[id.UserID][]time.Time `json:"resets"`
	Handled           []id.EventID              `json:"handled"`
}

// Refused is a request the bot won't or can't carry out; the message is for the helper
type Refused struct{ message string }

func (r Refused) Error() string { return r.message }

func refuse(format string, args ...any) error { return Refused{fmt.Sprintf(format, args...)} }

type Bot struct {
	ctx        context.Context
	cli        *mautrix.Client
	crypto     *cryptohelper.CryptoHelper
	options    Options
	helpers    map[id.UserID]Permissions
	server     string
	serverUser id.UserID
	adminRoom  id.RoomID

	mu      sync.Mutex
	state   State
	replies map[id.EventID]chan *event.Event
	work    chan *event.Event
}

func info(format string, args ...any) { log.Printf("[INFO] "+format, args...) }
func warn(format string, args ...any) { log.Printf("[WARN] "+format, args...) }

func main() {
	log.SetFlags(0)
	raw, err := os.ReadFile(optionsFile)
	if err != nil {
		log.Fatalf("[ERROR] Can't read the add-on options: %v", err)
	}
	var options Options
	if err := json.Unmarshal(raw, &options); err != nil {
		log.Fatalf("[ERROR] Can't read the add-on options: %v", err)
	}
	if options.Password == "" {
		log.Fatal("[ERROR] Set the bot account's 'password' on the Configuration tab (see the Documentation tab for setup).")
	}
	if options.MessageMinutes <= 0 {
		options.MessageMinutes = 10
	}
	options.Username = strings.SplitN(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(options.Username)), "@"), ":", 2)[0]
	if options.MatrixURL == "" {
		options.MatrixURL = defaultMatrixURL()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	for {
		err := run(ctx, options)
		if ctx.Err() != nil {
			return
		}
		warn("Lost contact with the Matrix server (%v); retrying in 15 seconds", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
}

// The Matrix add-on from the same repository: add-on hostnames are
// "<repository>-<slug>", so swap this add-on's slug for "matrix"
func defaultMatrixURL() string {
	hostname, _ := os.Hostname()
	if prefix, ok := strings.CutSuffix(hostname, "-matrix-helper-bot"); ok {
		return "http://" + prefix + "-matrix:8008"
	}
	return "http://localhost:8008"
}

func pickleKey() []byte {
	if key, err := os.ReadFile(pickleFile); err == nil && len(key) >= 32 {
		return key
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		log.Fatalf("[ERROR] No randomness: %v", err)
	}
	if err := os.WriteFile(pickleFile, key, 0o600); err != nil {
		log.Fatalf("[ERROR] Can't save the encryption key: %v", err)
	}
	return key
}

func run(parent context.Context, options Options) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	cli, err := mautrix.NewClient(options.MatrixURL, "", "")
	if err != nil {
		return err
	}
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, NoColor: true}).Level(zerolog.WarnLevel)
	cli.Log = logger

	helper, err := cryptohelper.NewCryptoHelper(cli, pickleKey(), cryptoDBFile)
	if err != nil {
		return err
	}
	helper.LoginAs = &mautrix.ReqLogin{
		Type:                     mautrix.AuthTypePassword,
		Identifier:               mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: options.Username},
		Password:                 options.Password,
		InitialDeviceDisplayName: "Matrix Helper Bot",
	}
	if err := helper.Init(ctx); err != nil {
		var httpErr mautrix.HTTPError
		if errors.As(err, &httpErr) && httpErr.RespError != nil && httpErr.RespError.StatusCode == 403 {
			log.Fatalf("[ERROR] Could not sign in as '%s': %s. Check 'username' and 'password' on the Configuration tab.",
				options.Username, httpErr.RespError.Err)
		}
		return fmt.Errorf("setting up encryption: %w", err)
	}
	defer helper.Close()
	cli.Crypto = helper
	machine := helper.Machine()
	machine.SendKeysMinTrust = minTrust
	machine.ShareKeysMinTrust = minTrust

	bot := &Bot{
		ctx:        ctx,
		cli:        cli,
		crypto:     helper,
		options:    options,
		server:     cli.UserID.Homeserver(),
		serverUser: id.NewUserID("conduit", cli.UserID.Homeserver()),
		replies:    map[id.EventID]chan *event.Event{},
		work:       make(chan *event.Event, 32),
	}
	bot.loadState()
	info("Signed in as %s (device %s)", cli.UserID, cli.DeviceID)

	if err := bot.crossSign(options.Password); err != nil {
		return fmt.Errorf("verifying the bot's device: %w", err)
	}
	bot.loadHelpers()
	bot.findAdminRoom()
	if err := bot.ensureHelpersRoom(); err != nil {
		return fmt.Errorf("setting up the Server helpers room: %w", err)
	}

	syncer := cli.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnSync(cli.DontProcessOldEvents)
	syncer.OnEventType(event.EventMessage, bot.onMessage)
	helper.DecryptErrorCallback = func(evt *event.Event, err error) {
		if evt.RoomID == bot.state.HelpersRoom {
			warn("Could not decrypt a message from %s: %v", evt.Sender, err)
		}
	}

	go bot.worker()
	go bot.housekeeping()
	info("Ready: %d helper(s) in the Server helpers room", len(bot.helpers))
	return cli.SyncWithContext(ctx)
}

// ----- state -----

func (b *Bot) loadState() {
	if raw, err := os.ReadFile(stateFile); err == nil {
		_ = json.Unmarshal(raw, &b.state)
	}
	if b.state.Resets == nil {
		b.state.Resets = map[id.UserID][]time.Time{}
	}
}

// saveState must be called with b.mu held
func (b *Bot) saveState() {
	raw, _ := json.Marshal(b.state)
	temporary := stateFile + ".new"
	if err := os.WriteFile(temporary, raw, 0o600); err == nil {
		_ = os.Rename(temporary, stateFile)
	}
}

// ----- setup -----

// crossSign gives the bot a cross-signing identity and signs its device with
// it, so helpers' apps treat the bot as a verified session
func (b *Bot) crossSign(password string) error {
	if b.state.CrossSignedDevice == b.cli.DeviceID {
		return nil
	}
	machine := b.crypto.Machine()
	if _, _, err := machine.GenerateAndUploadCrossSigningKeysWithPassword(b.ctx, password, ""); err != nil {
		return err
	}
	if err := machine.SignOwnDevice(b.ctx, machine.OwnIdentity()); err != nil {
		return err
	}
	if err := machine.SignOwnMasterKey(b.ctx); err != nil {
		return err
	}
	b.mu.Lock()
	b.state.CrossSignedDevice = b.cli.DeviceID
	b.saveState()
	b.mu.Unlock()
	info("Set up the bot's encryption identity and verified its device")
	return nil
}

func (b *Bot) loadHelpers() {
	b.helpers = map[id.UserID]Permissions{}
	for _, entry := range b.options.Helpers {
		user := strings.ToLower(strings.TrimSpace(entry.User))
		if !strings.HasPrefix(user, "@") {
			user = "@" + user
		}
		if !strings.Contains(user, ":") {
			user += ":" + b.server
		}
		localpart, server, _ := strings.Cut(user[1:], ":")
		if server != b.server || !localpartPattern.MatchString(localpart) || id.UserID(user) == b.cli.UserID {
			warn("Ignoring helper '%s': use an account on %s, like @name:%s", entry.User, b.server, b.server)
			continue
		}
		b.helpers[id.UserID(user)] = Permissions{ResetPasswords: entry.ResetPasswords}
	}
	if len(b.helpers) == 0 {
		warn("No helpers are listed on the Configuration tab, so nobody can use the bot yet.")
	}
}

func (b *Bot) findAdminRoom() {
	alias := id.NewRoomAlias("admins", b.server)
	resolved, err := b.cli.ResolveAlias(b.ctx, alias)
	if err == nil {
		room := resolved.RoomID
		if _, joined := b.members(room)[b.cli.UserID]; !joined {
			_, err = b.cli.JoinRoomByID(b.ctx, room)
		}
		if err == nil {
			b.adminRoom = room
			return
		}
	}
	b.adminRoom = ""
	warn("%s is not a server admin yet, so it can't reset passwords. In the Admin Room, send: !admin users make-user-admin %s",
		b.cli.UserID, b.cli.UserID)
}

func (b *Bot) members(room id.RoomID) map[id.UserID]mautrix.JoinedMember {
	if room == "" {
		return nil
	}
	resp, err := b.cli.JoinedMembers(b.ctx, room)
	if err != nil {
		return nil
	}
	return resp.Joined
}

func intPtr(value int) *int { return &value }

func (b *Bot) ensureHelpersRoom() error {
	room := b.state.HelpersRoom
	if room != "" {
		var create event.CreateEventContent
		if err := b.cli.StateEvent(b.ctx, room, event.StateCreate, "", &create); err != nil {
			room = ""
		}
	}
	if room == "" {
		invite := make([]id.UserID, 0, len(b.helpers))
		for user := range b.helpers {
			invite = append(invite, user)
		}
		slices.Sort(invite)
		encryption := &event.EncryptionEventContent{Algorithm: id.AlgorithmMegolmV1}
		resp, err := b.cli.CreateRoom(b.ctx, &mautrix.ReqCreateRoom{
			Name:       "Server helpers",
			Topic:      "Ask the helper bot to help with accounts. Type !help to see what you can do.",
			Preset:     "private_chat",
			Visibility: "private",
			Invite:     invite,
			InitialState: []*event.Event{
				{Type: event.StateEncryption, Content: event.Content{Parsed: encryption}},
				{Type: event.StateHistoryVisibility, Content: event.Content{Parsed: &event.HistoryVisibilityEventContent{HistoryVisibility: event.HistoryVisibilityJoined}}},
				{Type: event.StateGuestAccess, Content: event.Content{Parsed: &event.GuestAccessEventContent{GuestAccess: event.GuestAccessForbidden}}},
			},
			// Only the bot can invite, remove people or change the room
			PowerLevelOverride: &event.PowerLevelsEventContent{
				StateDefaultPtr: intPtr(100),
				InvitePtr:       intPtr(100),
				KickPtr:         intPtr(100),
				BanPtr:          intPtr(100),
				RedactPtr:       intPtr(100),
				Events:          map[string]int{event.StatePowerLevels.Type: 100},
			},
		})
		if err != nil {
			return err
		}
		room = resp.RoomID
		if store, ok := b.cli.StateStore.(interface {
			SetEncryptionEvent(context.Context, id.RoomID, *event.EncryptionEventContent) error
		}); ok {
			_ = store.SetEncryptionEvent(b.ctx, room, encryption)
		}
		b.mu.Lock()
		b.state.HelpersRoom = room
		b.saveState()
		b.mu.Unlock()
		info("Created the encrypted Server helpers room (%s)", room)
	}
	b.syncMembers()
	return nil
}

// syncMembers invites listed helpers who aren't in the room and removes anyone else
func (b *Bot) syncMembers() {
	room := b.state.HelpersRoom
	resp, err := b.cli.Members(b.ctx, room)
	if err != nil {
		warn("Could not check who is in the Server helpers room: %v", err)
		return
	}
	present := map[id.UserID]event.Membership{}
	for _, evt := range resp.Chunk {
		_ = evt.Content.ParseRaw(evt.Type)
		if member := evt.Content.AsMember(); member != nil && evt.StateKey != nil {
			present[id.UserID(*evt.StateKey)] = member.Membership
		}
	}
	for user := range b.helpers {
		if membership := present[user]; membership != event.MembershipJoin && membership != event.MembershipInvite {
			if _, err := b.cli.InviteUser(b.ctx, room, &mautrix.ReqInviteUser{UserID: user}); err != nil {
				warn("Could not invite %s: %v", user, err)
			} else {
				info("Invited %s to the Server helpers room", user)
			}
		}
	}
	for user, membership := range present {
		if _, listed := b.helpers[user]; user != b.cli.UserID && !listed &&
			(membership == event.MembershipJoin || membership == event.MembershipInvite) {
			if _, err := b.cli.KickUser(b.ctx, room, &mautrix.ReqKickUser{UserID: user, Reason: "Not listed as a helper in the Matrix Helper Bot settings"}); err == nil {
				info("Removed %s from the Server helpers room: not listed as a helper", user)
			}
		}
	}
}

func (b *Bot) housekeeping() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	lastMembers := time.Now()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
		}
		b.redactDue()
		if time.Since(lastMembers) > membershipInterval {
			b.syncMembers()
			lastMembers = time.Now()
		}
		if b.adminRoom == "" {
			b.findAdminRoom()
		}
	}
}

// ----- events -----

func (b *Bot) onMessage(_ context.Context, evt *event.Event) {
	switch {
	case evt.RoomID == b.adminRoom && evt.Sender == b.serverUser:
		replyTo := evt.Content.AsMessage().RelatesTo.GetReplyTo()
		b.mu.Lock()
		waiting := b.replies[replyTo]
		b.mu.Unlock()
		if waiting != nil {
			waiting <- evt
		}
	case evt.RoomID == b.state.HelpersRoom && evt.Sender != b.cli.UserID:
		select {
		case b.work <- evt:
		default:
			warn("Too many requests at once; ignoring one from %s", evt.Sender)
		}
	}
}

// worker handles commands one at a time, outside the sync loop, so it can
// wait for the server's replies
func (b *Bot) worker() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case evt := <-b.work:
			b.handle(evt)
		}
	}
}

func (b *Bot) handle(evt *event.Event) {
	content := evt.Content.AsMessage()
	if content.MsgType != event.MsgText && content.MsgType != event.MsgNotice {
		return
	}
	if content.RelatesTo != nil && content.RelatesTo.Type == event.RelReplace {
		return
	}
	words := strings.Fields(content.Body)
	if len(words) == 0 || !strings.HasPrefix(words[0], "!") {
		return
	}
	// Never act on the same message twice (sync can repeat after a restart)
	b.mu.Lock()
	if slices.Contains(b.state.Handled, evt.ID) {
		b.mu.Unlock()
		return
	}
	b.state.Handled = append(b.state.Handled, evt.ID)
	if len(b.state.Handled) > 200 {
		b.state.Handled = b.state.Handled[len(b.state.Handled)-200:]
	}
	b.saveState()
	b.mu.Unlock()

	if !evt.Mautrix.WasEncrypted || evt.Mautrix.TrustState < minTrust {
		warn("Ignored %s from %s: sent from a session they haven't verified", words[0], evt.Sender)
		b.say("I only take requests from verified sessions. Verify this device in your app (Settings → Security / Encryption), then try again.")
		return
	}

	command := strings.ToLower(words[0])
	var err error
	switch command {
	case "!help":
		b.help(evt.Sender)
	case "!reset-password", "!resetpassword", "!reset":
		err = b.reset(evt.Sender, content, words[1:])
	default:
		b.say(fmt.Sprintf("I don't know %s. Type !help to see what you can do.", words[0]))
	}
	var refusal Refused
	if errors.As(err, &refusal) {
		b.say(refusal.message)
	} else if err != nil {
		log.Printf("[ERROR] %s from %s failed: %v", command, evt.Sender, err)
		b.say("Something went wrong. The Matrix Helper Bot log has the details.")
	}
}

func (b *Bot) help(sender id.UserID) {
	lines := []string{"Things you can ask me:"}
	if b.helpers[sender].ResetPasswords {
		lines = append(lines, fmt.Sprintf("!reset-password @name — give someone a new password if they forgot theirs. "+
			"I post it here, and the message deletes itself after %d minutes.", b.options.MessageMinutes))
	}
	if len(lines) == 1 {
		lines = []string{"You don't have any permissions yet. The server admin can give you some in the Matrix Helper Bot settings."}
	}
	b.say(strings.Join(lines, "\n"))
}

var matrixToPattern = regexp.MustCompile(`https://matrix\.to/#/(@[^"?/<>]+)`)

// target is the account a command is about: a mention pill, or a typed name
func (b *Bot) target(content *event.MessageEventContent, words []string) (id.UserID, string, error) {
	var mentioned []string
	if content.Mentions != nil {
		for _, user := range content.Mentions.UserIDs {
			mentioned = append(mentioned, string(user))
		}
	}
	if len(mentioned) == 0 {
		for _, match := range matrixToPattern.FindAllStringSubmatch(content.FormattedBody, -1) {
			if user, err := url.PathUnescape(match[1]); err == nil {
				mentioned = append(mentioned, user)
			}
		}
	}
	slices.Sort(mentioned)
	mentioned = slices.Compact(mentioned)
	var user string
	switch {
	case len(mentioned) == 1:
		user = mentioned[0]
	case len(words) == 1:
		user = words[0]
	default:
		return "", "", refuse("Tell me one account, like: !reset-password @name")
	}
	user = strings.ToLower(user)
	if !strings.HasPrefix(user, "@") {
		user = "@" + user
	}
	if !strings.Contains(user, ":") {
		user += ":" + b.server
	}
	localpart, server, _ := strings.Cut(user[1:], ":")
	if server != b.server {
		return "", "", refuse("I can only help with accounts on %s.", b.server)
	}
	if !localpartPattern.MatchString(localpart) {
		return "", "", refuse("'%s' isn't a valid account name.", user)
	}
	return id.UserID(user), localpart, nil
}

func (b *Bot) reset(sender id.UserID, content *event.MessageEventContent, words []string) error {
	if !b.helpers[sender].ResetPasswords {
		return refuse("You don't have permission to reset passwords.")
	}
	if b.adminRoom == "" {
		return refuse("I'm not a server admin yet, so I can't reset passwords. Ask the server admin to set me up.")
	}
	user, localpart, err := b.target(content, words)
	if err != nil {
		return err
	}
	if user == b.cli.UserID || user == b.serverUser {
		return refuse("I can't reset that account.")
	}
	if _, isHelper := b.helpers[user]; isHelper {
		return refuse("I don't reset helpers' passwords. Ask the server admin.")
	}
	if _, isAdmin := b.members(b.adminRoom)[user]; isAdmin {
		return refuse("I don't reset server admins' passwords.")
	}
	b.mu.Lock()
	var recent []time.Time
	for _, stamp := range b.state.Resets[sender] {
		if time.Since(stamp) < time.Hour {
			recent = append(recent, stamp)
		}
	}
	b.mu.Unlock()
	if len(recent) >= resetsPerHour {
		return refuse("That's %d resets in the last hour. Try again later.", resetsPerHour)
	}

	password, err := b.resetPassword(localpart)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.state.Resets[sender] = append(recent, time.Now())
	b.saveState()
	b.mu.Unlock()
	info("%s reset the password of %s", sender, user)
	b.say(fmt.Sprintf("%s reset the password of %s.", sender, user))
	sent := b.say(fmt.Sprintf("New password for %s: %s\nGive it to them privately. This message deletes itself in %d minutes. "+
		"Their other signed-in devices stay signed in.", user, password, b.options.MessageMinutes))
	if sent != "" {
		b.mu.Lock()
		b.state.Redactions = append(b.state.Redactions, Redaction{
			Room: b.state.HelpersRoom, Event: sent, Due: time.Now().Add(time.Duration(b.options.MessageMinutes) * time.Minute),
		})
		b.saveState()
		b.mu.Unlock()
	}
	return nil
}

var successPattern = regexp.MustCompile("Successfully reset the password for user \\S+: `([^`\\s]+)`")

func (b *Bot) resetPassword(localpart string) (string, error) {
	reply, err := b.adminCommand("users reset-password " + localpart)
	if err != nil {
		return "", err
	}
	if password := b.passwordFrom(reply); password != "" {
		return password, nil
	}
	body := reply.Content.AsMessage().Body
	switch {
	case strings.Contains(body, "does not exist"):
		return "", refuse("There's no account called @%s:%s.", localpart, b.server)
	case strings.Contains(body, "shadow or deactivated"):
		// Accounts made by sidebar sign-in have no password ("shadow");
		// converting one is fine, but converting would also reactivate a
		// deactivated account, so tell them apart first
		if b.accountKind(localpart) != "shadow" {
			return "", refuse("@%s:%s is deactivated, so I won't reset it.", localpart, b.server)
		}
		reply, err = b.adminCommand("users reset-password --convert-to-local-account " + localpart)
		if err != nil {
			return "", err
		}
		if password := b.passwordFrom(reply); password != "" {
			return password, nil
		}
		body = reply.Content.AsMessage().Body
	}
	warn("Unexpected reply from the server: %.200s", body)
	return "", refuse("The server didn't reset the password. The Matrix Helper Bot log has the details.")
}

// passwordFrom reads the new password from the server's reply, then deletes the reply
func (b *Bot) passwordFrom(reply *event.Event) string {
	if _, err := b.cli.RedactEvent(b.ctx, b.adminRoom, reply.ID, mautrix.ReqRedact{Reason: "Contained a password"}); err != nil {
		warn("Could not delete the server's reply from the Admin Room: %v", err)
	}
	if match := successPattern.FindStringSubmatch(reply.Content.AsMessage().Body); match != nil {
		return match[1]
	}
	return ""
}

// accountKind is "deactivated", "shadow" (no password) or "password", from
// how the server refuses a sign-in with a random password
func (b *Bot) accountKind(localpart string) string {
	probe, err := mautrix.NewClient(b.options.MatrixURL, "", "")
	if err != nil {
		return "unknown"
	}
	_, err = probe.Login(b.ctx, &mautrix.ReqLogin{
		Type:       mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: localpart},
		Password:   randomHex(32),
	})
	var httpErr mautrix.HTTPError
	if errors.As(err, &httpErr) && httpErr.RespError != nil {
		switch {
		case httpErr.RespError.ErrCode == "M_USER_DEACTIVATED":
			return "deactivated"
		case strings.Contains(httpErr.RespError.Err, "cannot log in with a password"):
			return "shadow"
		}
	}
	return "password"
}

// adminCommand sends an admin command and returns the server's reply
func (b *Bot) adminCommand(command string) (*event.Event, error) {
	sent, err := b.cli.SendMessageEvent(b.ctx, b.adminRoom, event.EventMessage,
		&event.MessageEventContent{MsgType: event.MsgText, Body: "!admin " + command})
	if err != nil {
		return nil, err
	}
	waiting := make(chan *event.Event, 1)
	b.mu.Lock()
	b.replies[sent.EventID] = waiting
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.replies, sent.EventID)
		b.mu.Unlock()
	}()
	select {
	case reply := <-waiting:
		return reply, nil
	case <-time.After(30 * time.Second):
		return nil, refuse("The server didn't answer. Try again in a minute.")
	case <-b.ctx.Done():
		return nil, b.ctx.Err()
	}
}

// ----- messages -----

func randomHex(n int) string {
	raw := make([]byte, n)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}

// say posts an (encrypted) notice in the helpers room and returns its event ID
func (b *Bot) say(text string) id.EventID {
	resp, err := b.cli.SendMessageEvent(b.ctx, b.state.HelpersRoom, event.EventMessage,
		&event.MessageEventContent{MsgType: event.MsgNotice, Body: text})
	if err != nil {
		warn("Could not post in the Server helpers room: %v", err)
		return ""
	}
	return resp.EventID
}

func (b *Bot) redactDue() {
	b.mu.Lock()
	var due, remaining []Redaction
	for _, item := range b.state.Redactions {
		if time.Now().After(item.Due) {
			due = append(due, item)
		} else {
			remaining = append(remaining, item)
		}
	}
	b.mu.Unlock()
	for _, item := range due {
		if _, err := b.cli.RedactEvent(b.ctx, item.Room, item.Event, mautrix.ReqRedact{Reason: "Password message expired"}); err != nil {
			warn("Could not delete an old password message: %v", err)
			remaining = append(remaining, item)
		}
	}
	if len(due) > 0 {
		b.mu.Lock()
		b.state.Redactions = remaining
		b.saveState()
		b.mu.Unlock()
	}
}
