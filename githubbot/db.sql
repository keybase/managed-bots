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
  `branch` varchar(128) NOT NULL,
  `hook_id` bigint(20) NOT NULL,
  `oauth_identifier` varchar(128) NOT NULL,
  UNIQUE KEY unique_subscription (`conv_id`, `repo`, `branch`) 
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `user_prefs` (
  `username` varchar(128) NOT NULL,
  `mention` tinyint(1) NOT NULL,
  PRIMARY KEY (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
