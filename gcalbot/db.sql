DROP TABLE IF EXISTS `accounts`;

CREATE TABLE `accounts` (
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

DROP TABLE IF EXISTS `subscriptions`;

CREATE TABLE `subscriptions` (
    `username` varchar(128) NOT NULL,
    `nickname` varchar(128) NOT NULL,
    `calendar` varchar(128) NOT NULL,
    `type` ENUM('invites', 'updates', 'notifications') NOT NULL,
    PRIMARY KEY(`username`, `nickname`, `calendar`, `type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
