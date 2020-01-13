CREATE TABLE `hooks` (
  `id` varchar(100) NOT NULL,
  `name` varchar(100) NOT NULL,
  `conv_id` varchar(100) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `conv_id` (`conv_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;