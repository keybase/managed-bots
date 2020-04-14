CREATE TABLE `macro` (
  `channel_name` varchar(128) NOT NULL,
  `macro_name` varchar(128) NOT NULL,
  `macro_message` varchar(128) NOT NULL,
  PRIMARY KEY (`channel_name`, `macro_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
