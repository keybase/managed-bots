CREATE TABLE `heartbeats` (
  `id` varchar(32) NOT NULL,
  `name` varchar(50) NOT NULL,
  `mtime` datetime(6) NOT NULL,
  PRIMARY KEY (`id`, `name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;