CREATE TABLE `deferrals` (
  `id` int NOT NULL AUTO_INCREMENT,
  `regex` text NOT NULL,
  `author` varchar(50) NOT NULL DEFAULT '',
  `ctime` datetime NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
