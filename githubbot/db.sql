CREATE TABLE `oauth` (
  `identifier` varchar(128) NOT NULL,
  `ctime` datetime NOT NULL,
  `mtime` datetime NOT NULL,
  `access_token` varchar(256) NOT NULL,
  `token_type` varchar(64) NOT NULL,
  PRIMARY KEY (`identifier`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `subscriptions` (
  `conv_id` varchar(20) NOT NULL,
  `repo` varchar(128) NOT NULL,
  `branch` varchar(128) NOT NULL,
  `hook_id` bigint(20) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `user_prefs` (
  `username` varchar(128) NOT NULL,
  `mention` tinyint(1) NOT NULL,
  PRIMARY KEY (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;