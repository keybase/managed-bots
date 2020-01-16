DROP TABLE IF EXISTS `accounts`;

CREATE TABLE `account` (
  `username` varchar(128) NOT NULL,
  `nickname` varchar(128) NOT NULL,
  PRIMARY KEY (`username`, `nickname`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `oauth`;

CREATE TABLE `oauth` (
  `identifier` varchar(128) UNIQUE NOT NULL,
  `ctime` datetime NOT NULL,
  `mtime` datetime NOT NULL,
  `access_token` varchar(256) NOT NULL,
  `token_type` varchar(64) NOT NULL,
  `refresh_token` varchar(256) NOT NULL,
  `expiry` datetime NOT NULL,
  PRIMARY KEY (`identifier`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `invite`;

CREATE TABLE `invite` (
    `username` varchar(128) NOT NULL,
    `nickname` varchar(128) NOT NULL,
    `calendar_id` varchar(128) NOT NULL,
    `event_id` varchar(128) NOT NULL,
    `message_id` int(11) unsigned NOT NULL,
    PRIMARY KEY(`username`, `nickname`, `event_id`),
    UNIQUE KEY(`username`, `message_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
