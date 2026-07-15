CREATE TABLE `deferrals` (
  `id` int NOT NULL AUTO_INCREMENT,
  `team` varchar(255) NOT NULL,
  `regex` text NOT NULL,
  `author` varchar(50) NOT NULL DEFAULT '',
  `ctime` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_team` (`team`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
