<?php

/**
 * @file
 * An API key for scripts that sync with farmOS (run by run.sh through Drush once farmOS is installed).
 *
 * An "API sync" account with the Farm Manager role (its random password is never used) and an OAuth
 * client, api_sync, that can only get Farm Manager tokens through the client credentials grant. The
 * client's id and a random secret are written to /config/api_client.json, the add-on's own folder
 * (addon_configs on the host), readable only there. Deleting the file makes a new secret at the next
 * start.
 */

declare(strict_types=1);

use Drupal\consumers\Entity\Consumer;
use Drupal\user\Entity\User;

const KEY_FILE = '/config/api_client.json';
const CLIENT_ID = 'api_sync';
const NAME = 'API sync';

$etm = \Drupal::entityTypeManager();
$found = $etm->getStorage('consumer')->loadByProperties(['client_id' => CLIENT_ID]);
$client = $found ? reset($found) : NULL;
if ($client && is_file(KEY_FILE)) {
  echo "API key: ready\n";
  return;
}

// The account the client acts as.
$found = $etm->getStorage('user')->loadByProperties(['name' => NAME]);
$account = $found ? reset($found) : User::create([
  'name' => NAME,
  'status' => 1,
  'pass' => bin2hex(random_bytes(32)),
]);
if (!$account->hasRole('farm_manager')) {
  $account->addRole('farm_manager');
}
$account->save();

$secret = bin2hex(random_bytes(32));
$client ??= Consumer::create([
  'client_id' => CLIENT_ID,
  'label' => NAME,
  'grant_types' => ['client_credentials'],
  'scopes' => ['farm_manager'],
  'confidential' => TRUE,
  'third_party' => FALSE,
  'user_id' => $account->id(),
  'access_token_expiration' => 3600,
]);
$client->set('secret', $secret);
$client->save();

$old = umask(0077);
file_put_contents(KEY_FILE, json_encode(['client_id' => CLIENT_ID, 'client_secret' => $secret], JSON_PRETTY_PRINT) . "\n");
umask($old);
chmod(KEY_FILE, 0600);
echo "API key: made a new one for the " . CLIENT_ID . " client, in " . KEY_FILE . "\n";
