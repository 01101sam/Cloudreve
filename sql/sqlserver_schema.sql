-- Cloudreve v4 – SQL Server schema (manual migration)
-- ----------------------------------------------------
-- NOTE: This script is generated from ent/migrate/schema.go.
--       Review it carefully before applying to production.
--
-- 1.  You MUST run it in an empty database that uses a case-sensitive collation.
-- 2.  All JSON fields are stored as NVARCHAR(MAX) but you can change them to
--     SQL Server's native JSON type when it becomes available.
-- 3.  All *_at columns use DATETIME2; adjust precision if needed.
-- 4.  Foreign-key constraints follow the same ON DELETE rules defined in Ent.
-- 5.  After running this script, insert the required default rows (settings,
--     storage policy, groups, etc.) by starting Cloudreve once – it will skip
--     schema creation but still add data when the tables are present.

SET ANSI_NULLS ON;
SET QUOTED_IDENTIFIER ON;
GO

/*--------------------------------------------
  nodes
--------------------------------------------*/
IF OBJECT_ID('nodes', 'U') IS NULL
BEGIN
    CREATE TABLE nodes (
        id               INT IDENTITY(1,1)      PRIMARY KEY,
        created_at        DATETIME2              NOT NULL,
        updated_at        DATETIME2              NOT NULL,
        deleted_at        DATETIME2              NULL,
        status            NVARCHAR(16)           NOT NULL,
        name              NVARCHAR(255)          NOT NULL,
        type              NVARCHAR(16)           NOT NULL,
        server            NVARCHAR(255)          NULL,
        slave_key         NVARCHAR(255)          NULL,
        capabilities      VARBINARY(MAX)         NOT NULL,
        settings          VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC NULL,
        weight            INT                    NOT NULL DEFAULT 0
    );
END;
GO

/*--------------------------------------------
  storage_policies
--------------------------------------------*/
IF OBJECT_ID('storage_policies', 'U') IS NULL
BEGIN
    CREATE TABLE storage_policies (
        id                INT IDENTITY(1,1)      PRIMARY KEY,
        created_at         DATETIME2              NOT NULL,
        updated_at         DATETIME2              NOT NULL,
        deleted_at         DATETIME2              NULL,
        name               NVARCHAR(255)          NOT NULL,
        type               NVARCHAR(64)           NOT NULL,
        server             NVARCHAR(255)          NULL,
        bucket_name        NVARCHAR(255)          NULL,
        is_private         BIT                    NULL,
        access_key         NVARCHAR(MAX)          NULL,
        secret_key         NVARCHAR(MAX)          NULL,
        max_size           BIGINT                 NULL,
        dir_name_rule      NVARCHAR(512)          NULL,
        file_name_rule     NVARCHAR(512)          NULL,
        settings           VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC NULL,
        node_id            INT                    NULL,
        CONSTRAINT fk_storage_policies_node FOREIGN KEY (node_id)
            REFERENCES nodes(id) ON DELETE SET NULL
    );
END;
GO

/*--------------------------------------------
  groups
--------------------------------------------*/
IF OBJECT_ID('groups', 'U') IS NULL
BEGIN
    CREATE TABLE groups (
        id                INT IDENTITY(1,1)      PRIMARY KEY,
        created_at         DATETIME2              NOT NULL,
        updated_at         DATETIME2              NOT NULL,
        deleted_at         DATETIME2              NULL,
        name               NVARCHAR(255)          NOT NULL,
        max_storage        BIGINT                 NULL,
        speed_limit        INT                    NULL,
        permissions        VARBINARY(MAX)         NOT NULL,
        settings           VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC NULL,
        storage_policy_id  INT                    NULL,
        CONSTRAINT fk_groups_storage_policy FOREIGN KEY (storage_policy_id)
            REFERENCES storage_policies(id) ON DELETE SET NULL
    );
END;
GO

/*--------------------------------------------
  users
--------------------------------------------*/
IF OBJECT_ID('users', 'U') IS NULL
BEGIN
    CREATE TABLE users (
        id                 INT IDENTITY(1,1)     PRIMARY KEY,
        created_at          DATETIME2             NOT NULL,
        updated_at          DATETIME2             NOT NULL,
        deleted_at          DATETIME2             NULL,
        email               NVARCHAR(100)         NOT NULL UNIQUE,
        nick                NVARCHAR(100)         NOT NULL,
        password            NVARCHAR(255)         NULL,
        status              NVARCHAR(16)          NOT NULL DEFAULT 'active',
        storage             BIGINT                NOT NULL DEFAULT 0,
        two_factor_secret   NVARCHAR(255)         NULL,
        avatar              NVARCHAR(255)         NULL,
        settings            VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC NULL,
        group_users         INT                   NOT NULL,
        CONSTRAINT fk_users_group FOREIGN KEY (group_users)
            REFERENCES groups(id) ON DELETE NO ACTION
    );
END;
GO

/*--------------------------------------------
  settings
--------------------------------------------*/
IF OBJECT_ID('settings', 'U') IS NULL
BEGIN
    CREATE TABLE settings (
        id          INT IDENTITY(1,1) PRIMARY KEY,
        created_at  DATETIME2         NOT NULL,
        updated_at  DATETIME2         NOT NULL,
        deleted_at  DATETIME2         NULL,
        name        NVARCHAR(255)     NOT NULL UNIQUE,
        value       VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC     NULL
    );
END;
GO

/*--------------------------------------------
  tasks
--------------------------------------------*/
IF OBJECT_ID('tasks', 'U') IS NULL
BEGIN
    CREATE TABLE tasks (
        id                  INT IDENTITY(1,1)      PRIMARY KEY,
        created_at          DATETIME2              NOT NULL,
        updated_at          DATETIME2              NOT NULL,
        deleted_at          DATETIME2              NULL,
        status              NVARCHAR(32)           NOT NULL,
        type                NVARCHAR(64)           NOT NULL,
        private_state       VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC          NULL,
        public_state        VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC          NULL,
        retried             INT                    NOT NULL DEFAULT 0,
        executed            BIGINT                 NOT NULL DEFAULT 0,
        last_iteration_at   DATETIME2              NULL,
        suspend_until       BIGINT                 NULL,
        error               VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC          NULL,
        correlation_id      NVARCHAR(36)           NULL,
        user_tasks          INT                    NULL,
        CONSTRAINT fk_tasks_user FOREIGN KEY (user_tasks)
            REFERENCES users(id) ON DELETE CASCADE
    );

    CREATE INDEX idx_tasks_status ON tasks(status);
    CREATE INDEX idx_tasks_type   ON tasks(type);
    CREATE INDEX idx_tasks_user   ON tasks(user_tasks);
END;
GO

/*--------------------------------------------
  passkeys
--------------------------------------------*/
IF OBJECT_ID('passkeys', 'U') IS NULL
BEGIN
    CREATE TABLE passkeys (
        id                  INT IDENTITY(1,1)      PRIMARY KEY,
        created_at          DATETIME2              NOT NULL,
        updated_at          DATETIME2              NOT NULL,
        deleted_at          DATETIME2              NULL,
        user_id             INT                    NOT NULL,
        credential_id       NVARCHAR(255)          NOT NULL,
        name                NVARCHAR(255)          NOT NULL,
        credential          VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC  NULL,
        used_at             DATETIME2              NULL,
        CONSTRAINT fk_passkeys_user FOREIGN KEY (user_id)
            REFERENCES users(id) ON DELETE CASCADE,
        CONSTRAINT uq_passkeys_user_credential UNIQUE (user_id, credential_id)
    );

    CREATE INDEX idx_passkeys_user ON passkeys(user_id);
END;
GO

/*--------------------------------------------
  entities
--------------------------------------------*/
IF OBJECT_ID('entities', 'U') IS NULL
BEGIN
    CREATE TABLE entities (
        id                       INT IDENTITY(1,1)      PRIMARY KEY,
        created_at               DATETIME2              NOT NULL,
        updated_at               DATETIME2              NOT NULL,
        deleted_at               DATETIME2              NULL,
        type                     INT                    NOT NULL,
        source                   NVARCHAR(MAX)          NOT NULL,
        size                     BIGINT                 NOT NULL,
        reference_count          INT                    NOT NULL DEFAULT 1,
        storage_policy_entities  INT                    NOT NULL,
        created_by               INT                    NULL,
        upload_session_id        UNIQUEIDENTIFIER       NULL,
        recycle_options          VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC NULL,
        CONSTRAINT fk_entities_storage_policy FOREIGN KEY (storage_policy_entities)
            REFERENCES storage_policies(id) ON DELETE NO ACTION,
        CONSTRAINT fk_entities_user FOREIGN KEY (created_by)
            REFERENCES users(id) ON DELETE SET NULL
    );

    CREATE INDEX idx_entities_policy ON entities(storage_policy_entities);
END;
GO

/*--------------------------------------------
  metadata
--------------------------------------------*/
IF OBJECT_ID('metadata', 'U') IS NULL
BEGIN
    CREATE TABLE metadata (
        id               INT IDENTITY(1,1)      PRIMARY KEY,
        created_at        DATETIME2              NOT NULL,
        updated_at        DATETIME2              NOT NULL,
        deleted_at        DATETIME2              NULL,
        name              NVARCHAR(255)          NOT NULL,
        value             VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC NOT NULL,
        file_id           INT                    NOT NULL,
        is_public         BIT                    NOT NULL DEFAULT 0,
        CONSTRAINT uq_metadata_file_name UNIQUE (file_id, name)
    );

    CREATE INDEX idx_metadata_file ON metadata(file_id);
END;
GO

/*--------------------------------------------
  files
--------------------------------------------*/
IF OBJECT_ID('files', 'U') IS NULL
BEGIN
    CREATE TABLE files (
        id                     INT IDENTITY(1,1)      PRIMARY KEY,
        created_at             DATETIME2              NOT NULL,
        updated_at             DATETIME2              NOT NULL,
        deleted_at             DATETIME2              NULL,
        name                   NVARCHAR(255)          NOT NULL,
        type                   INT                    NOT NULL,
        size                   BIGINT                 NOT NULL DEFAULT 0,
        owner_id               INT                    NOT NULL,
        file_children          INT                    NULL,
        storage_policy_files   INT                    NULL,
        primary_entity         INT                    NULL,
        is_symbolic            BIT                    NOT NULL DEFAULT 0,
        props                  VARCHAR(MAX) COLLATE Latin1_General_100_CI_AS_SC NULL,
        CONSTRAINT fk_files_owner FOREIGN KEY (owner_id)
            REFERENCES users(id) ON DELETE CASCADE,
        CONSTRAINT fk_files_parent FOREIGN KEY (file_children)
            REFERENCES files(id) ON DELETE NO ACTION,
        CONSTRAINT fk_files_storage_policy FOREIGN KEY (storage_policy_files)
            REFERENCES storage_policies(id) ON DELETE SET NULL,
        CONSTRAINT fk_files_primary_entity FOREIGN KEY (primary_entity)
            REFERENCES entities(id) ON DELETE SET NULL
    );

    CREATE INDEX idx_files_owner   ON files(owner_id);
    CREATE INDEX idx_files_parent  ON files(file_children);
END;
GO

/* Add foreign key now that files table exists */
IF OBJECT_ID('files', 'U') IS NOT NULL AND OBJECT_ID('metadata', 'U') IS NOT NULL AND NOT EXISTS (SELECT 1 FROM sys.foreign_keys WHERE name = 'fk_metadata_file')
BEGIN
    ALTER TABLE metadata
        WITH CHECK ADD CONSTRAINT fk_metadata_file FOREIGN KEY(file_id)
        REFERENCES files(id) ON DELETE CASCADE;
END;
GO

-- Additional tables (entities, shares, etc.) follow the same
-- pattern. For brevity they are omitted here – generate them with the helper
-- tool or continue translating from ent/migrate/schema.go.
-- ----------------------------------------------------
-- End of schema 