CREATE TABLE `polls` (
  `conv_id` varchar(100) NOT NULL,
  `msg_id` int(11) NOT NULL,
  `result_msg_id` int(11) NOT NULL,
  `choices` int(11) NOT NULL,
  PRIMARY KEY (`conv_id`,`msg_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;


CREATE TABLE `votes` (
  `conv_id` varchar(100) NOT NULL,
  `msg_id` int(11) NOT NULL,
  `username` varchar(50) NOT NULL,
  `choice` int(11) NOT NULL,
  PRIMARY KEY (`conv_id`,`msg_id`,`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;