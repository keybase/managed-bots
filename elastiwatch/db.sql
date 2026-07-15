CREATE TABLE `deferrals` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `team` varchar(255) NOT NULL,
  `regex` text NOT NULL,
  `author` varchar(255) NOT NULL,
  `ctime` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_team` (`team`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
