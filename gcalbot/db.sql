DROP TABLE IF EXISTS `oauth_state`;

CREATE TABLE `oauth_state` (
  `state` char(24) NOT NULL,
  `keybase_username` varchar(128) NOT NULL,
  `account_nickname` varchar(128) NOT NULL,
  `keybase_conv_id` char(64) NOT NULL,
  `is_complete` boolean NOT NULL DEFAULT 0,
  PRIMARY KEY (`state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `account`;

CREATE TABLE `account` (
    `keybase_username` varchar(128) NOT NULL,   -- kb username
    `account_nickname` varchar(128) NOT NULL,   -- nickname of google account for kb user
    `ctime` datetime NOT NULL,
    `mtime` datetime NOT NULL,
    `access_token` varchar(256) NOT NULL,
    `token_type` varchar(64) NOT NULL,
    `refresh_token` varchar(256) NOT NULL,
    `expiry` datetime NOT NULL,
    PRIMARY KEY (`keybase_username`, `account_nickname`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `channel`;

CREATE TABLE `channel` (
    `channel_id` varchar(128) NOT NULL,         -- unique id for webhook channel
    `keybase_username` varchar(128) NOT NULL,   -- kb username
    `account_nickname` varchar(128) NOT NULL,   -- nickname of google account for kb user
    `calendar_id` varchar(128) NOT NULL,        -- google calendar id that this channel is watching
    `resource_id` varchar(128) NOT NULL,        -- google resource id that this channel is watching (events for the calendar)
    `expiry` datetime NOT NULL,                 -- when the webhook channel expires
    `next_sync_token` varchar(128) NOT NULL,    -- token used for incremental syncs on each webhook
    PRIMARY KEY (`channel_id`),
    UNIQUE KEY (`keybase_username`, `account_nickname`, `calendar_id`),
    FOREIGN KEY (`keybase_username`, `account_nickname`)
        REFERENCES account(`keybase_username`, `account_nickname`)
        ON DELETE CASCADE,
    INDEX (`expiry`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `subscription`;

CREATE TABLE `subscription` (
    `keybase_username` varchar(128) NOT NULL,       -- kb username
    `account_nickname` varchar(128) NOT NULL,       -- nickname of google account for kb user
    `calendar_id` varchar(128) NOT NULL,            -- google calendar id that this subscription is for
    `keybase_conv_id` char(64) NOT NULL,            -- channel that is subscribed to notifications
    `minutes_before` int(11) NOT NULL DEFAULT 0,    -- minutes until event that a notification should be sent (for reminder)
    `type` ENUM ('invite', 'reminder'),             -- type of subscription
    PRIMARY KEY (`keybase_username`, `account_nickname`, `calendar_id`, `keybase_conv_id`, `minutes_before`, `type`),
    FOREIGN KEY (`keybase_username`, `account_nickname`)
        REFERENCES account(`keybase_username`, `account_nickname`)
        ON DELETE CASCADE,
    FOREIGN KEY (`keybase_username`, `account_nickname`, `calendar_id`)
        REFERENCES channel(`keybase_username`, `account_nickname`, `calendar_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `invite`;

CREATE TABLE `invite` (
    `keybase_username` varchar(128) NOT NULL,   -- kb username
    `account_nickname` varchar(128) NOT NULL,   -- nickname of google account for kb user
    `calendar_id` varchar(128) NOT NULL,        -- google calendar id that this invite is for
    `event_id` varchar(128) NOT NULL,           -- id of the event that the account is invited to
    `message_id` int(11) unsigned NOT NULL,     -- message id of the keybase message invite sent to the user over chat
    PRIMARY KEY (`keybase_username`, `account_nickname`, `calendar_id`, `event_id`),
    UNIQUE KEY (`keybase_username`, `message_id`),
    FOREIGN KEY (`keybase_username`, `account_nickname`)
        REFERENCES account(`keybase_username`, `account_nickname`)
        ON DELETE CASCADE
    -- no foreign key to subscription, want to keep invites after unsubscribe so that users can still react to invites
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `daily_schedule_subscription`;

CREATE TABLE `daily_schedule_subscription` (
    `keybase_username` varchar(128) NOT NULL,       -- kb username
    `account_nickname` varchar(128) NOT NULL,       -- nickname of google account for kb user
    `calendar_id` varchar(128) NOT NULL,            -- google calendar id that this subscription is for
    `keybase_conv_id` char(64) NOT NULL,            -- channel that is subscribed to notifications
    `days_to_send` ENUM ('everyday', 'monday through friday', 'sunday through thursday'), -- days of the week to send notifications
    `schedule_to_send` ENUM ('today', 'tomorrow'),  -- schedule to send
    `notification_duration` int(11) NOT NULL,       -- minutes after beginning of UTC day before notification should be sent
    PRIMARY KEY (`keybase_username`, `account_nickname`, `calendar_id`, `keybase_conv_id`),
    FOREIGN KEY (`keybase_username`, `account_nickname`)
        REFERENCES account(`keybase_username`, `account_nickname`)
        ON DELETE CASCADE,
    INDEX (`notification_duration`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
