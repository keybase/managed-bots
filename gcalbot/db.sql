DROP TABLE IF EXISTS `account`;

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

DROP TABLE IF EXISTS `channel`;

CREATE TABLE `channel` (
    `id` varchar(128) NOT NULL,
    `username` varchar(128) NOT NULL,
    `nickname` varchar(128) NOT NULL,
    `calendar_id` varchar(128) NOT NULL,
    `resource_id` varchar(128) NOT NULL,
    `next_sync_token` varchar(128) NOT NULL,
    PRIMARY KEY(`id`),
    UNIQUE KEY(`username`, `nickname`, `calendar_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `subscription`;

CREATE TABLE `subscription` (
    `username` varchar(128) NOT NULL,
    `nickname` varchar(128) NOT NULL,
    `calendar_id` varchar(128) NOT NULL,
    `type` ENUM('invite'),
    PRIMARY KEY(`username`, `nickname`, `calendar_id`)
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
