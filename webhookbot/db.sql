CREATE TABLE `hooks` (
  `id` varchar(100) NOT NULL,
  `name` varchar(100) NOT NULL,
  `conv_id` varchar(100) NOT NULL,
  `template` varchar(10000) NOT NULL,
  CONSTRAINT hook UNIQUE (`name`,`conv_id`),
  PRIMARY KEY (`id`),
  KEY `conv_id` (`conv_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
