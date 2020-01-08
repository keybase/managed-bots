DROP TABLE IF EXISTS `subscriptions`;

CREATE TABLE `subscriptions` (
  `conv_id` varchar(20) NOT NULL,
  `repo` varchar(128) NOT NULL,
  `branch` varchar(128) NOT NULL,
  `hook_id` bigint NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

DROP TABLE IF EXISTS `oauth`;

CREATE TABLE `oauth` (
  `identifier` varchar(128) NOT NULL,
  `ctime` datetime NOT NULL,
  `mtime` datetime NOT NULL,
  `access_token` varchar(256) NOT NULL,
  `token_type` varchar(64) NOT NULL,
  PRIMARY KEY (`identifier`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
