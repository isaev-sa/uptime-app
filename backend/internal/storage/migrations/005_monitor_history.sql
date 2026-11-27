ALTER TABLE monitors ADD COLUMN revision bigint NOT NULL DEFAULT 1,
                     ADD COLUMN url_version bigint NOT NULL DEFAULT 1;

CREATE TABLE monitor_minutes (
    monitor_id uuid NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    minute timestamptz NOT NULL,
    successes integer NOT NULL,
    failures integer NOT NULL,
    last_batch uuid NOT NULL,
    PRIMARY KEY (monitor_id, minute)
);
CREATE INDEX monitor_minutes_expiry ON monitor_minutes(minute);

CREATE FUNCTION revise_monitor() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.revision := OLD.revision + 1;
    IF NEW.url IS DISTINCT FROM OLD.url THEN
        NEW.url_version := OLD.url_version + 1;
        DELETE FROM monitor_minutes WHERE monitor_id = OLD.id;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER revise_monitor BEFORE UPDATE ON monitors
FOR EACH ROW EXECUTE FUNCTION revise_monitor();

CREATE FUNCTION notify_monitor_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_notify('monitor_changes', '');
    RETURN NULL;
END $$;
CREATE TRIGGER monitor_changed AFTER INSERT OR UPDATE OR DELETE ON monitors
FOR EACH STATEMENT EXECUTE FUNCTION notify_monitor_change();
