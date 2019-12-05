DROP TABLE IF EXISTS `subscriptions`;

CREATE TABLE `subscriptions` (
  `conv_id` varchar(20) NOT NULL,
  `repo` varchar(128) NOT NULL,
  `branch` varchar(128) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
