CREATE TABLE users(
	id                    VARBINARY(192) NOT NULL,
	access_token          BLOB NOT NULL,
	refresh_token         VARBINARY(255) NOT NULL,
	expires_at            DATETIME       NOT NULL,
	name                  VARCHAR(255)   NOT NULL,
	email                 VARCHAR(255)   NOT NULL,
	last_seen_activity_id BIGINT,
	created_at            DATETIME       NOT NULL,
	updated_at            DATETIME       NOT NULL,
	PRIMARY KEY(id)
) DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE subscriptions(
	token                 VARBINARY(50)  NOT NULL,
	user_id               VARBINARY(192) NOT NULL,
	playlist_id           VARBINARY(192) NOT NULL,
	playlist_owner_id     VARBINARY(192) NOT NULL,
	playlist_name         VARCHAR(255)   NOT NULL,
	playlist_version      VARBINARY(192) NOT NULL,
	playlist_tracks       BLOB           NOT NULL,
	next_check_at         DATETIME,
	created_at            DATETIME       NOT NULL,
	updated_at            DATETIME       NOT NULL,
	PRIMARY KEY(token),
	UNIQUE KEY(user_id, playlist_id),
	INDEX(playlist_id)
) DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE activities(
	id                 BIGINT         NOT NULL AUTO_INCREMENT,
	subscription_token VARCHAR(50)    NOT NULL,
	unique_id          VARBINARY(255) NOT NULL,
	user_id            VARBINARY(192) NOT NULL,
	data               BLOB           NOT NULL,
	created_at         DATETIME       NOT NULL,
	PRIMARY KEY(id),
	UNIQUE KEY(unique_id),
	INDEX(user_id),
	INDEX(subscription_token)
) DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
