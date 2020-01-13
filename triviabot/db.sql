CREATE TABLE `leaderboard` (
  `conv_id` varchar(100) NOT NULL,
  `username` varchar(100) NOT NULL,
  `points` int(11) NOT NULL DEFAULT '0',
  `correct` int(11) NOT NULL DEFAULT '0',
  `incorrect` int(11) NOT NULL DEFAULT '0',
  PRIMARY KEY (`conv_id`,`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `tokens` (
  `conv_id` varchar(100) NOT NULL,
  `token` varchar(100) NOT NULL,
  PRIMARY KEY (`conv_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;