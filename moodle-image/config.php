<?php
// This file is a template adapted for the Moodle LMS Operator.
// It reads configuration from environment variables, which are injected
// by the operator based on the MoodleTenant Custom Resource.

unset($CFG);
global $CFG;
$CFG = new stdClass();

// --- Database Configuration ---
// These values are injected from a Secret created by the operator.
$CFG->dbtype    = 'pgsql';
$CFG->dblibrary = 'native';
$CFG->dbhost    = getenv('DB_HOST');
$CFG->dbname    = getenv('DB_NAME');
$CFG->dbuser    = getenv('DB_USER');
$CFG->dbpass    = getenv('DB_PASS');
$CFG->prefix    = 'mdl_';

// --- Site URL and Data Root ---
// MOODLE_URL is derived from the `spec.hostname` of the CR.
$CFG->sslproxy = true;
$CFG->wwwroot   = getenv('MOODLE_URL');
// The data root is a fixed path inside the container, mounted to a PVC.
$CFG->dataroot  = '/var/www/moodledata'; 
$CFG->admin     = 'admin';

// --- Performance & Caching (Sidecar Pattern) ---
// Use file-based sessions instead of memcached for simplicity
$CFG->session_handler_class = '\core\session\file';
$CFG->session_file_save_path = $CFG->dataroot.'/sessions';

// Optional: Configure MUC (Moodle Universal Cache) to also use Memcached
// $CFG->memcached_servers = array( '127.0.0.1' => '11211' );
// $CFG->localcache_memcached_server = '127.0.0.1:11211';
// $CFG->localcache_memcached_store = '\cachestore_memcached\store';

// --- Security & Other Settings ---
$CFG->passwordsaltmain = getenv('MOODLE_PASSWORD_SALT') ?: 'default-salt-please-change';

// Do not send email from test environments
$CFG->noemailever = true;

// Any other custom settings can be added below this line.

//=========================================================================
// ALL DONE!  To continue installation, visit your main page with a browser
//=========================================================================

require_once(__DIR__ . '/lib/setup.php'); // Do not edit
