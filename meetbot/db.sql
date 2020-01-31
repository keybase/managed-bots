CREATE TABLE `oauth_state` (
  `state` char(24) NOT NULL,
  `identifier` varchar(128) NOT NULL,
  `conv_id` char(64) NOT NULL,
  `msg_id` char(64) NOT NULL,
  PRIMARY KEY (`state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `oauth` (
  `identifier` varchar(128) NOT NULL,
  `ctime` datetime NOT NULL,
  `mtime` datetime NOT NULL,
  `access_token` varchar(256) NOT NULL,
  `token_type` varchar(64) NOT NULL,
  `refresh_token` varchar(256) NOT NULL,
  `expiry` datetime NOT NULL,
  PRIMARY KEY (`identifier`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
