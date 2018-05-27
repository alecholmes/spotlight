ALTER TABLE activities
  MODIFY COLUMN subscription_token VARBINARY(50) NOT NULL,
  DROP INDEX unique_id,
  ADD UNIQUE INDEX user_id_unique_id (user_id, unique_id);
