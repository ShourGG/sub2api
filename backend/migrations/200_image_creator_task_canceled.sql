ALTER TABLE image_creator_tasks
    DROP CONSTRAINT IF EXISTS chk_image_creator_tasks_status;

ALTER TABLE image_creator_tasks
    ADD CONSTRAINT chk_image_creator_tasks_status
    CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'canceled'));
