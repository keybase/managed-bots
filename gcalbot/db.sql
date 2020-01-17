DROP TABLE IF EXISTS `account`;

CREATE TABLE `account` (
    `account_id` varchar(128) NOT NULL,
    `keybase_username` varchar(128) NOT NULL,
    `account_nickname` varchar(128) NOT NULL,
    PRIMARY KEY (`account_id`),
    UNIQUE KEY (`keybase_username`, `account_nickname`)
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
    -- oauth row is created before account row, so can't have foreign key reference to account
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `channel`;

CREATE TABLE `channel` (
    `channel_id` varchar(128) NOT NULL,
    `account_id` varchar(128) NOT NULL,
    `calendar_id` varchar(128) NOT NULL,
    `resource_id` varchar(128) NOT NULL,
    `expiry` datetime NOT NULL,
    `next_sync_token` varchar(128) NOT NULL,
    PRIMARY KEY (`channel_id`),
    UNIQUE KEY (`account_id`, `calendar_id`),
    FOREIGN KEY (`account_id`) REFERENCES account(`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `subscription`;

CREATE TABLE `subscription` (
    `account_id` varchar(128) NOT NULL,
    `calendar_id` varchar(128) NOT NULL,
    `type` ENUM ('invite'),
    PRIMARY KEY (`account_id`, `calendar_id`),
    FOREIGN KEY (`account_id`) REFERENCES account(`account_id`),
    FOREIGN KEY (`account_id`, `calendar_id`) REFERENCES channel(`account_id`, `calendar_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `invite`;

CREATE TABLE `invite` (
    `account_id` varchar(128) NOT NULL,
    `calendar_id` varchar(128) NOT NULL,
    `event_id` varchar(128) NOT NULL,
    `keybase_username` varchar(128) NOT NULL,
    `message_id` int(11) unsigned NOT NULL,
    PRIMARY KEY (`account_id`, `calendar_id`, `event_id`),
    UNIQUE KEY (`keybase_username`, `message_id`),
    FOREIGN KEY (`account_id`) REFERENCES account(`account_id`)
    -- no foreign key to subscription, want to keep invites after unsubscribe so that users can still react to invites
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
