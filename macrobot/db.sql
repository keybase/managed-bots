CREATE TABLE `macro` (
  `channel_name` varchar(128) NOT NULL,
  `macro_name` varchar(128) NOT NULL,
  `macro_message` varchar(10000) NOT NULL,
  -- NOTE: if `is_conv` is set, `channel_name` holds a `conversation_id`
  `is_conv` BOOLEAN DEFAULT FALSE NOT NULL,
  PRIMARY KEY (`channel_name`, `macro_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
