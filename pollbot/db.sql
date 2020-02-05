
CREATE TABLE `polls` (
  `id` varchar(16) NOT NULL,
  `conv_id` varchar(100) NOT NULL,
  `msg_id` int(11) NOT NULL,
  `result_msg_id` int(11) NOT NULL,
  `choices` int(11) DEFAULT NULL,
   PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `votes` (
  `id` varchar(16) NOT NULL,
  `username` varchar(50) NOT NULL,
  `choice` int(11) NOT NULL,
   PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;