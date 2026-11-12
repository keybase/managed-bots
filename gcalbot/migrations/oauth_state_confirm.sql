-- Run before deploying the OAuth link confirmation change.

-- keybase_username is renamed to requester so the confirmation steps can be
-- shared with the other bots (base.OAuthStateDB).
ALTER TABLE `oauth_state`
  CHANGE COLUMN `keybase_username` `requester` varchar(128) NOT NULL,
  ADD COLUMN `ctime` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ADD COLUMN `pending_token` text,
  ADD COLUMN `confirm_code_hash` char(64),
  ADD COLUMN `confirm_attempts` int NOT NULL DEFAULT 0,
  ADD INDEX (`requester`);

-- Drop links started before this change; they predate the expiry check.
DELETE FROM `oauth_state`;
