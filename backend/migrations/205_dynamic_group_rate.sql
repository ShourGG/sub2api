-- Dynamic group pricing derives the public multiplier from the highest
-- upstream-account multiplier currently attached to a group. The result is
-- always rounded upward to 0.005 so the group cannot undercharge after a rate
-- change. Existing groups remain manual until explicitly enabled.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS dynamic_rate_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS dynamic_rate_markup DECIMAL(10,4) NOT NULL DEFAULT 1.25,
    ADD COLUMN IF NOT EXISTS dynamic_rate_source_multiplier DECIMAL(10,4) NOT NULL DEFAULT 0;

ALTER TABLE groups
    DROP CONSTRAINT IF EXISTS groups_dynamic_rate_markup_positive;
ALTER TABLE groups
    ADD CONSTRAINT groups_dynamic_rate_markup_positive CHECK (dynamic_rate_markup > 0);

CREATE OR REPLACE FUNCTION recalculate_dynamic_group_rate(target_group_id BIGINT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    enabled BOOLEAN;
    markup NUMERIC;
    source_multiplier NUMERIC;
    resolved_multiplier NUMERIC;
BEGIN
    SELECT dynamic_rate_enabled, dynamic_rate_markup
    INTO enabled, markup
    FROM groups
    WHERE id = target_group_id AND deleted_at IS NULL;

    IF NOT FOUND OR NOT enabled THEN
        UPDATE groups
        SET dynamic_rate_source_multiplier = 0,
            updated_at = NOW()
        WHERE id = target_group_id
          AND dynamic_rate_source_multiplier IS DISTINCT FROM 0;
        RETURN;
    END IF;

    SELECT MAX(a.rate_multiplier)
    INTO source_multiplier
    FROM account_groups ag
    JOIN accounts a ON a.id = ag.account_id
    WHERE ag.group_id = target_group_id
      AND a.deleted_at IS NULL;

    -- A newly-created or temporarily unbound group retains its last manual
    -- sell rate. This avoids silently turning an unavailable group free.
    IF source_multiplier IS NULL THEN
        UPDATE groups
        SET dynamic_rate_source_multiplier = 0,
            updated_at = NOW()
        WHERE id = target_group_id
          AND dynamic_rate_source_multiplier IS DISTINCT FROM 0;
        RETURN;
    END IF;

    resolved_multiplier := CEIL((source_multiplier * markup) / 0.005) * 0.005;
    UPDATE groups
    SET rate_multiplier = resolved_multiplier,
        dynamic_rate_source_multiplier = source_multiplier,
        updated_at = NOW()
    WHERE id = target_group_id
      AND (rate_multiplier IS DISTINCT FROM resolved_multiplier
           OR dynamic_rate_source_multiplier IS DISTINCT FROM source_multiplier);
END;
$$;

CREATE OR REPLACE FUNCTION trg_recalculate_dynamic_group_rate_from_group()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM recalculate_dynamic_group_rate(NEW.id);
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION trg_recalculate_dynamic_group_rate_from_account()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    affected_group_id BIGINT;
BEGIN
    FOR affected_group_id IN
        SELECT group_id FROM account_groups WHERE account_id = NEW.id
    LOOP
        PERFORM recalculate_dynamic_group_rate(affected_group_id);
    END LOOP;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION trg_recalculate_dynamic_group_rate_from_account_group()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM recalculate_dynamic_group_rate(OLD.group_id);
        RETURN OLD;
    END IF;
    PERFORM recalculate_dynamic_group_rate(NEW.group_id);
    IF TG_OP = 'UPDATE' AND OLD.group_id IS DISTINCT FROM NEW.group_id THEN
        PERFORM recalculate_dynamic_group_rate(OLD.group_id);
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_groups_recalculate_dynamic_rate ON groups;
CREATE TRIGGER trg_groups_recalculate_dynamic_rate
AFTER INSERT OR UPDATE ON groups
FOR EACH ROW EXECUTE FUNCTION trg_recalculate_dynamic_group_rate_from_group();

DROP TRIGGER IF EXISTS trg_accounts_recalculate_dynamic_group_rate ON accounts;
CREATE TRIGGER trg_accounts_recalculate_dynamic_group_rate
AFTER UPDATE OF rate_multiplier, deleted_at ON accounts
FOR EACH ROW
WHEN (OLD.rate_multiplier IS DISTINCT FROM NEW.rate_multiplier
      OR OLD.deleted_at IS DISTINCT FROM NEW.deleted_at)
EXECUTE FUNCTION trg_recalculate_dynamic_group_rate_from_account();

DROP TRIGGER IF EXISTS trg_account_groups_recalculate_dynamic_rate ON account_groups;
CREATE TRIGGER trg_account_groups_recalculate_dynamic_rate
AFTER INSERT OR DELETE OR UPDATE OF account_id, group_id ON account_groups
FOR EACH ROW EXECUTE FUNCTION trg_recalculate_dynamic_group_rate_from_account_group();
