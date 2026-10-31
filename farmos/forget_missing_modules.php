<?php

/**
 * @file
 * Forgets installed modules whose code is gone (run by run.sh through Drush before the database updates).
 *
 * A module that a newer farmOS no longer ships (Field Kit, gone in farmOS 4) stays listed as installed.
 * Drupal then refuses to install any other module, so an update that does (farmOS 4's OAuth split) fails
 * and the remaining updates never run, leaving the site in maintenance mode. As Drupal recommends
 * (https://www.drupal.org/node/3137656), drop such a module from the installed list and its schema
 * version. Its leftover configuration stays and does no harm.
 */

declare(strict_types=1);

$extension = \Drupal::configFactory()->getEditable('core.extension');
$installed = $extension->get('module') ?? [];
// Scanned afresh: the cached list (still farmOS 3's after an upgrade) would include the missing ones
$available = \Drupal::service('extension.list.module')->reset()->getList();

$missing = array_keys(array_diff_key($installed, $available));
if (!$missing) {
  return;
}

foreach ($missing as $module) {
  unset($installed[$module]);
  \Drupal::keyValue('system.schema')->delete($module);
}
$extension->set('module', $installed)->save();
echo 'Forgot modules that are no longer in farmOS: ' . implode(', ', $missing) . "\n";
