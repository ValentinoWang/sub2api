//go:build integration

package repository

// The two prior function fixtures are copied from the sanitized development
// and production schema snapshots captured before the alignment migration.

const migration240PriorDevFunctions = `
CREATE OR REPLACE FUNCTION public.enforce_deepmath_openai_ws_policy_on_account() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.platform IS DISTINCT FROM 'openai'
        OR NEW.type NOT IN ('oauth', 'apikey')
        OR NOT EXISTS (
            SELECT 1
            FROM account_groups AS ag
            JOIN groups AS g ON g.id = ag.group_id
            WHERE ag.account_id = NEW.id
              AND g.deleted_at IS NULL
              AND lower(btrim(g.name)) = 'deepmath'
        ) THEN
        RETURN NEW;
    END IF;

    NEW.extra := COALESCE(NEW.extra, '{}'::jsonb);
    IF NEW.type = 'oauth' THEN
        NEW.extra := jsonb_set(
            NEW.extra,
            '{openai_oauth_responses_websockets_v2_mode}',
            to_jsonb('ctx_pool'::text),
            true
        );
    ELSE
        NEW.extra := jsonb_set(
            NEW.extra,
            '{openai_apikey_responses_websockets_v2_mode}',
            to_jsonb('ctx_pool'::text),
            true
        );
    END IF;
    NEW.extra := jsonb_set(
        NEW.extra,
        '{openai_ws_allow_store_recovery}',
        'true'::jsonb,
        true
    );
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION public.enforce_deepmath_openai_ws_policy_on_membership() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    changed_account_id BIGINT;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM groups AS g
        WHERE g.id = NEW.group_id
          AND g.deleted_at IS NULL
          AND lower(btrim(g.name)) = 'deepmath'
    ) THEN
        RETURN NEW;
    END IF;

    UPDATE accounts AS a
    SET extra = CASE a.type
        WHEN 'oauth' THEN jsonb_set(
            jsonb_set(COALESCE(a.extra, '{}'::jsonb),
                '{openai_oauth_responses_websockets_v2_mode}',
                to_jsonb('ctx_pool'::text), true),
            '{openai_ws_allow_store_recovery}', 'true'::jsonb, true)
        WHEN 'apikey' THEN jsonb_set(
            jsonb_set(COALESCE(a.extra, '{}'::jsonb),
                '{openai_apikey_responses_websockets_v2_mode}',
                to_jsonb('ctx_pool'::text), true),
            '{openai_ws_allow_store_recovery}', 'true'::jsonb, true)
        ELSE a.extra
    END,
    updated_at = NOW()
    WHERE a.id = NEW.account_id
      AND a.platform = 'openai'
      AND a.type IN ('oauth', 'apikey')
      AND (
          a.extra->>'openai_ws_allow_store_recovery' IS DISTINCT FROM 'true'
          OR (a.type = 'oauth'
              AND a.extra->>'openai_oauth_responses_websockets_v2_mode' IS DISTINCT FROM 'ctx_pool')
          OR (a.type = 'apikey'
              AND a.extra->>'openai_apikey_responses_websockets_v2_mode' IS DISTINCT FROM 'ctx_pool')
      )
    RETURNING a.id INTO changed_account_id;

    IF changed_account_id IS NOT NULL THEN
        INSERT INTO scheduler_outbox (event_type, account_id)
        VALUES ('account_changed', changed_account_id);
    END IF;
    RETURN NEW;
END;
$$;
`

const migration240PriorProdFunctions = `
CREATE OR REPLACE FUNCTION public.enforce_deepmath_openai_ws_policy_on_account() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.platform IS DISTINCT FROM 'openai'
        OR NEW.type NOT IN ('oauth', 'apikey')
        OR NOT EXISTS (
            SELECT 1
            FROM account_groups AS ag
            JOIN groups AS g ON g.id = ag.group_id
            WHERE ag.account_id = NEW.id
              AND g.deleted_at IS NULL
              AND lower(btrim(g.name)) = 'deepmath'
        ) THEN
        RETURN NEW;
    END IF;

    NEW.extra := COALESCE(NEW.extra, '{}'::jsonb);
    IF NEW.type = 'oauth' THEN
        NEW.extra := jsonb_set(
            NEW.extra,
            '{openai_oauth_responses_websockets_v2_mode}',
            to_jsonb('ctx_pool'::text), true
        );
        NEW.extra := jsonb_set(NEW.extra, '{openai_ws_allow_store_recovery}', 'false'::jsonb, true);
    ELSE
        NEW.extra := jsonb_set(
            NEW.extra,
            '{openai_apikey_responses_websockets_v2_mode}',
            to_jsonb('ctx_pool'::text), true
        );
        NEW.extra := jsonb_set(NEW.extra, '{openai_ws_allow_store_recovery}', 'true'::jsonb, true);
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION public.enforce_deepmath_openai_ws_policy_on_membership() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    changed_account_id BIGINT;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM groups AS g
        WHERE g.id = NEW.group_id
          AND g.deleted_at IS NULL
          AND lower(btrim(g.name)) = 'deepmath'
    ) THEN
        RETURN NEW;
    END IF;

    UPDATE accounts AS a
    SET extra = CASE a.type
        WHEN 'oauth' THEN jsonb_set(
            jsonb_set(COALESCE(a.extra, '{}'::jsonb),
                '{openai_oauth_responses_websockets_v2_mode}',
                to_jsonb('ctx_pool'::text), true),
            '{openai_ws_allow_store_recovery}', 'false'::jsonb, true)
        WHEN 'apikey' THEN jsonb_set(
            jsonb_set(COALESCE(a.extra, '{}'::jsonb),
                '{openai_apikey_responses_websockets_v2_mode}',
                to_jsonb('ctx_pool'::text), true),
            '{openai_ws_allow_store_recovery}', 'true'::jsonb, true)
        ELSE a.extra
    END,
    updated_at = NOW()
    WHERE a.id = NEW.account_id
      AND a.platform = 'openai'
      AND a.type IN ('oauth', 'apikey')
      AND (
          a.extra->>'openai_ws_allow_store_recovery' IS DISTINCT FROM
              CASE a.type WHEN 'apikey' THEN 'true' ELSE 'false' END
          OR (a.type = 'oauth'
              AND a.extra->>'openai_oauth_responses_websockets_v2_mode' IS DISTINCT FROM 'ctx_pool')
          OR (a.type = 'apikey'
              AND a.extra->>'openai_apikey_responses_websockets_v2_mode' IS DISTINCT FROM 'ctx_pool')
      )
    RETURNING a.id INTO changed_account_id;

    IF changed_account_id IS NOT NULL THEN
        INSERT INTO scheduler_outbox (event_type, account_id)
        VALUES ('account_changed', changed_account_id);
    END IF;
    RETURN NEW;
END;
$$;
`
