CREATE TABLE `oauth_state` (
  `state` char(24) NOT NULL,
  `identifier` varchar(128) NOT NULL,
  `conv_id` char(64) NOT NULL,
  `msg_id` char(64) NOT NULL,
  `is_complete` boolean NOT NULL DEFAULT 0,
  PRIMARY KEY (`state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `oauth` (
  `identifier` varchar(128) NOT NULL,
  `ctime` datetime NOT NULL,
  `mtime` datetime NOT NULL,
  `access_token` varchar(256) NOT NULL,
  `token_type` varchar(64) NOT NULL,
  PRIMARY KEY (`identifier`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `subscriptions` (
  `conv_id` char(64) NOT NULL,
  `repo` varchar(128) NOT NULL,
  `installation_id` bigint(20) NOT NULL,
  UNIQUE KEY unique_subscription (`conv_id`, `repo`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `branches` (
  `conv_id` char(64) NOT NULL,
  `repo` varchar(128) NOT NULL,
  `branch` varchar(128) NOT NULL,
  UNIQUE KEY unique_subscription (`conv_id`, `repo`, `branch`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `features` (
  `conv_id` char(64) NOT NULL,
  `repo` varchar(128) NOT NULL,
  `issues` boolean NOT NULL DEFAULT 1,
  `pull_requests` boolean NOT NULL DEFAULT 1,
  `commits` boolean NOT NULL DEFAULT 0,
  `statuses` boolean NOT NULL DEFAULT 1,
  `releases` boolean NOT NULL DEFAULT 1,
  UNIQUE KEY unique_subscription (`conv_id`, `repo`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `user_prefs` (
  `username` varchar(128) NOT NULL,
  `conv_id` char(64) NOT NULL,
  `mention` tinyint(1) NOT NULL,
  PRIMARY KEY unique_prefs (`username`, `conv_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
