CREATE TABLE `subscriptions` (
  `conv_id` char(64) NOT NULL,
  `repo` varchar(128) NOT NULL,
  `branch` varchar(128) NOT NULL,
  `oauth_identifier` varchar(128) NOT NULL,
  UNIQUE KEY unique_subscription (`conv_id`, `repo`, `branch`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `notified_branches` (
  `conv_id` char(64) NOT NULL,
  `repo` varchar(128) NOT NULL,
  `branch` varchar(128) NOT NULL,
  `ctime` datetime NOT NULL,
  UNIQUE KEY unique_notified_branch (`conv_id`, `repo`, `branch`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
