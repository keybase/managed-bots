DROP TABLE IF EXISTS `account`;

CREATE TABLE `account` (
    `account_id` varchar(128) NOT NULL,         -- unique id per connected google account (currently kbuser:nickname)
    `keybase_username` varchar(128) NOT NULL,   -- kb username
    `account_nickname` varchar(128) NOT NULL,   -- nickname of google account for kb user
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
    -- oauth row is created before account row, so currently can't have foreign key reference to account
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `channel`;

CREATE TABLE `channel` (
    `channel_id` varchar(128) NOT NULL,         -- unique id for webhook channel
    `account_id` varchar(128) NOT NULL,         -- id for kb & google account (references account table)
    `calendar_id` varchar(128) NOT NULL,        -- google calendar id that this channel is watching
    `resource_id` varchar(128) NOT NULL,        -- google resource id that this channel is watching (events for the calendar)
    `expiry` datetime NOT NULL,                 -- when the webhook channel expires
    `next_sync_token` varchar(128) NOT NULL,    -- token used for incremental syncs on each webhook
    PRIMARY KEY (`channel_id`),
    UNIQUE KEY (`account_id`, `calendar_id`),
    FOREIGN KEY (`account_id`) REFERENCES account(`account_id`),
    INDEX (`expiry`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `subscription`;

CREATE TABLE `subscription` (
    `account_id` varchar(128) NOT NULL,         -- id for kb & google account (references account table)
    `calendar_id` varchar(128) NOT NULL,        -- google calendar id that this subscription is for
    `type` ENUM ('invite'),                     -- type of subscription
    PRIMARY KEY (`account_id`, `calendar_id`),
    FOREIGN KEY (`account_id`) REFERENCES account(`account_id`),
    FOREIGN KEY (`account_id`, `calendar_id`) REFERENCES channel(`account_id`, `calendar_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `invite`;

CREATE TABLE `invite` (
    `account_id` varchar(128) NOT NULL,         -- id for kb & google account (references account table)
    `calendar_id` varchar(128) NOT NULL,        -- google calendar id that this invite is for
    `event_id` varchar(128) NOT NULL,           -- id of the event that the account is invited to
    -- keybase_username can't be replaced with join on the account table, as on a react we might not know the account nickname, just the username & message id
    `keybase_username` varchar(128) NOT NULL,   -- username associated with the invite
    `message_id` int(11) unsigned NOT NULL,     -- message id of the keybase message invite sent to the user over chat
    PRIMARY KEY (`account_id`, `calendar_id`, `event_id`),
    UNIQUE KEY (`keybase_username`, `message_id`),
    FOREIGN KEY (`account_id`) REFERENCES account(`account_id`)
    -- no foreign key to subscription, want to keep invites after unsubscribe so that users can still react to invites
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
