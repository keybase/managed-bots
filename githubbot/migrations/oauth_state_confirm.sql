-- Run before deploying the OAuth link confirmation change.

ALTER TABLE `oauth_state`
  ADD COLUMN `requester` varchar(128) NOT NULL DEFAULT '' AFTER `identifier`,
  ADD COLUMN `ctime` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ADD COLUMN `pending_token` text,
  ADD COLUMN `confirm_code_hash` char(64),
  ADD COLUMN `confirm_attempts` int NOT NULL DEFAULT 0,
  ADD INDEX (`requester`);

-- Links started before this change have no requester and can't be confirmed.
DELETE FROM `oauth_state`;
