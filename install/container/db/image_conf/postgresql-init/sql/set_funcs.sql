/*
    Onix CMDB - Copyright (c) 2018-2019 by www.gatblau.org

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0
    Unless required by applicable law or agreed to in writing, software distributed under
    the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
    either express or implied.
    See the License for the specific language governing permissions and limitations under the License.

    Contributors to this project, hereby assign copyright in this code to the project,
    to be licensed under the same terms as the rest of the code.

*/
DO $$
BEGIN

/*
  set_item(...)
  Inserts a new or updates an existing item.
  Concurrency Management:
   - If the item is found in the database, the function attempts an update of the existing record.
      In this case, if a null value is passed as local_version_param, no optimistic locking is performed.
      If a value is specified for local_version_param, the update is only performed if and only if the version in the database matches the passed in version.
   - If the item is not found in the database, then the local_version_param is ignored and a record with version 1 is inserted.
 */
CREATE OR REPLACE FUNCTION set_item(
    key_param character varying,
    name_param character varying,
    description_param text,
    meta_param jsonb,
    tag_param text[],
    attribute_param hstore,
    status_param smallint,
    item_type_key_param character varying,
    local_version_param bigint,
    changedby_param character varying,
    transaction_ref_param uuid
  )
  RETURNS TABLE(result char(1))
  LANGUAGE 'plpgsql'
  COST 100
  VOLATILE
AS $BODY$
  DECLARE
    result char(1); -- the result status for the upsert
    current_version bigint; -- the version of the row before the update or null if no row
    rows_affected integer;
    item_type_id_value integer;
  BEGIN
    -- find the item type surrogate key from the provided natural key
    SELECT id FROM item_type WHERE key = item_type_key_param INTO item_type_id_value;
    IF (item_type_id_value IS NULL) THEN
      -- the provided natural key is not in the item type table, cannot proceed
     RAISE EXCEPTION 'Item Type Key --> % not found.', item_type_key_param
        USING hint = 'Check an Item Type with the key exist in the database.';
    END IF;

    -- checks that the attributes passed in comply with the validation in the item_type
    PERFORM check_item_attr(item_type_key_param, attribute_param);

    -- get the item current version
    SELECT version FROM item WHERE key = key_param INTO current_version;
    -- if no version is found then go for an insert
    IF (current_version IS NULL) THEN
      INSERT INTO item (
        id,
        key,
        name,
        description,
        meta,
        tag,
        attribute,
        status,
        item_type_id,
        version,
        created,
        updated,
        changedby,
        transaction_ref
      )
      VALUES (
          nextval('item_id_seq'),
          key_param,
          name_param,
          description_param,
          meta_param,
          tag_param,
          attribute_param,
          status_param,
          item_type_id_value,
          1,
          current_timestamp,
          null,
          changedby_param,
          transaction_ref_param
      );
      result := 'I';
    ELSE
      -- if a version is found, go for an update
      UPDATE item SET
        name = name_param,
        description = description_param,
        meta = meta_param,
        tag = tag_param,
        attribute = attribute_param,
        status = status_param,
        item_type_id = item_type_id_value,
        version = version + 1,
        updated = current_timestamp,
        changedby = changedby_param,
        transaction_ref = transaction_ref_param
      WHERE key = key_param
      -- the database record has not been modified by someone else
      -- if a null value is passed as local version then it does not perform optimistic locking
      AND (local_version_param = current_version OR local_version_param IS NULL)
      AND (
        -- the fields to be updated have not changed
        name != name_param OR
        description != description_param OR
        status != status_param OR
        item_type_id != item_type_id_value OR
        meta != meta_param OR
        tag != tag_param OR
        attribute != attribute_param OR
        transaction_ref != transaction_ref_param
      );
      -- determines if the update has gone ahead
      GET DIAGNOSTICS rows_affected := ROW_COUNT;
      -- works out the update status
      SELECT get_update_status(current_version, local_version_param, rows_affected > 0) INTO result;
    END IF;
    RETURN QUERY SELECT result;
  END;
  $BODY$;

ALTER FUNCTION set_item(character varying,character varying,text,jsonb, text[],hstore,smallint,character varying, bigint, character varying, uuid)
OWNER TO onix;

/*
  set_item_type(...)
  Inserts a new or updates an existing item item.
  Concurrency Management:
   - If the item type is found in the database, the function attempts an update of the existing record.
      In this case, if a null value is passed as local_version_param, no optimistic locking is performed.
      If a value is specified for local_version_param, the update is only performed if and only if the version in the database matches the passed in version.
   - If the item type is not found in the database, then the local_version_param is ignored and a record with version 1 is inserted.
 */
CREATE OR REPLACE FUNCTION set_item_type(
    key_param character varying,
    name_param character varying,
    description_param text,
    attr_valid_param hstore, -- keys allowed or required in item attributes
    local_version_param bigint,
    changedby_param character varying
  )
  RETURNS TABLE(result char(1))
  LANGUAGE 'plpgsql'
  COST 100
  VOLATILE
AS $BODY$
  DECLARE
    result char(1); -- the result status for the upsert
    current_version bigint; -- the version of the row before the update or null if no row
    rows_affected integer;
BEGIN
  -- checks that the attribute store parameter contain the correct values
  PERFORM check_attr_valid(attr_valid_param);

  SELECT version FROM item_type WHERE key = key_param INTO current_version;
  IF (current_version IS NULL) THEN
    INSERT INTO item_type (
      id,
      key,
      name,
      description,
      attr_valid,
      version,
      created,
      updated,
      changedby
    )
    VALUES (
      nextval('item_type_id_seq'),
      key_param,
      name_param,
      description_param,
      attr_valid_param,
      1,
      current_timestamp,
      null,
      changedby_param
    );
    result := 'I';
  ELSE
    UPDATE item_type SET
      name = name_param,
      description = description_param,
      attr_valid = attr_valid_param,
      version = version + 1,
      updated = current_timestamp,
      changedby = changedby_param
    WHERE key = key_param
    -- concurrency management - optimistic locking
    AND (local_version_param = current_version OR local_version_param IS NULL)
    AND (
      name != name_param OR
      description != description_param OR
      attr_valid != attr_valid_param
    );
    GET DIAGNOSTICS rows_affected := ROW_COUNT;
    SELECT get_update_status(current_version, local_version_param, rows_affected > 0) INTO result;
  END IF;
  RETURN QUERY SELECT result;
END;
$BODY$;

ALTER FUNCTION set_item_type(character varying, character varying, text, hstore, bigint, character varying)
OWNER TO onix;

/*
  set_link_type(...)
  Inserts a new or updates an existing link type.
  Concurrency Management:
   - If the link type is found in the database, the function attempts an update of the existing record.
      In this case, if a null value is passed as local_version_param, no optimistic locking is performed.
      If a value is specified for local_version_param, the update is only performed if and only if the version in the database matches the passed in version.
   - If the link type is not found in the database, then the local_version_param is ignored and a record with version 1 is inserted.
 */
CREATE OR REPLACE FUNCTION set_link_type(
    key_param character varying,
    name_param character varying,
    description_param text,
    attr_valid_param hstore, -- keys allowed or required in item attributes
    local_version_param bigint,
    changedby_param character varying
  )
  RETURNS TABLE(result char(1))
  LANGUAGE 'plpgsql'
  COST 100
  VOLATILE
AS $BODY$
DECLARE
  result char(1); -- the result status for the upsert
  current_version bigint; -- the version of the row before the update or null if no row
  rows_affected integer;
BEGIN
  -- checks that the attribute store parameter contain the correct values
  PERFORM check_attr_valid(attr_valid_param);

  -- gets the link type current version
  SELECT version FROM link_type WHERE key = key_param INTO current_version;

  IF (current_version IS NULL) THEN
    INSERT INTO link_type (
      id,
      key,
      name,
      description,
      attr_valid,
      version,
      created,
      updated,
      changedby
    )
    VALUES (
      nextval('link_type_id_seq'),
      key_param,
      name_param,
      description_param,
      attr_valid_param,
      1,
      current_timestamp,
      null,
      changedby_param
    );
    result := 'I';
  ELSE
    UPDATE link_type SET
       name = name_param,
       description = description_param,
       attr_valid = attr_valid_param,
       version = version + 1,
       updated = current_timestamp,
       changedby = changedby_param
    WHERE key = key_param
    -- concurrency management - optimistic locking
    AND (local_version_param = current_version OR local_version_param IS NULL)
    AND (
      name != name_param OR
      description != description_param OR
      attr_valid != attr_valid_param
    );
    GET DIAGNOSTICS rows_affected := ROW_COUNT;
    SELECT get_update_status(current_version, local_version_param, rows_affected > 0) INTO result;
  END IF;
  RETURN QUERY SELECT result;
END;
$BODY$;

ALTER FUNCTION set_link_type(character varying, character varying, text, hstore, bigint, character varying)
OWNER TO onix;

/*
  set_link(...)
  Inserts a new or updates an existing link.
  Concurrency Management:
   - If the link is found in the database, the function attempts an update of the existing record.
      In this case, if a null value is passed as local_version_param, no optimistic locking is performed.
      If a value is specified for local_version_param, the update is only performed if and only if the version in the database matches the passed in version.
   - If the link is not found in the database, then the local_version_param is ignored and a record with version 1 is inserted.
 */
CREATE OR REPLACE FUNCTION set_link(
    key_param character varying,
    link_type_key_param character varying,
    start_item_key_param character varying,
    end_item_key_param character varying,
    description_param text,
    meta_param jsonb,
    tag_param text[],
    attribute_param hstore,
    local_version_param bigint,
    changedby_param character varying,
    transaction_ref_param uuid
  )
  RETURNS TABLE(result char(1))
  LANGUAGE 'plpgsql'
  COST 100
  VOLATILE
AS $BODY$
  DECLARE
    result char(1); -- the result status for the upsert
    current_version bigint; -- the version of the row before the update or null if no row
    rows_affected integer;
    start_item_id_value bigint;
    end_item_id_value bigint;
    link_type_id_value integer;
    start_item_type_key_value character varying;
    end_item_type_key_value character varying;
BEGIN
  -- find the link type surrogate key from the provided natural key
  SELECT id FROM link_type WHERE key = link_type_key_param INTO link_type_id_value;
  IF (link_type_id_value IS NULL) THEN
    -- the provided natural key is not in the link type table, cannot proceed
    RAISE EXCEPTION 'Link Type Key --> % not found.', link_type_key_param
      USING hint = 'Check a Link Type with the key exist in the database.';
  END IF;

  SELECT i.id, t.key
  FROM item i
    INNER JOIN item_type t
      ON i.item_type_id = t.id
  WHERE i.key = start_item_key_param
    INTO start_item_id_value, start_item_type_key_value;

  IF (start_item_id_value IS NULL) THEN
    -- the start item does not exist
    RAISE EXCEPTION 'Start item with key --> % does not exist.', start_item_key_param
      USING hint = 'Check an item with the specified key exist in the database.';
  END IF;

  SELECT i.id, t.key
  FROM item i
    INNER JOIN item_type t
      ON i.item_type_id = t.id
  WHERE i.key = end_item_key_param
    INTO end_item_id_value, end_item_type_key_value;

  IF (end_item_id_value IS NULL) THEN
    -- the end item does not exist
    RAISE EXCEPTION 'End item with key --> % does not exist.', end_item_key_param
      USING hint = 'Check an item with the specified key exist in the database.';
  END IF;

  -- checks that the link is allowed
  PERFORM check_link(link_type_key_param, start_item_type_key_value, end_item_type_key_value);

  -- checks that the attributes passed in comply with the validation in the link_type
  PERFORM check_link_attr(link_type_key_param, attribute_param);

  SELECT version FROM link WHERE key = key_param INTO current_version;
  IF (current_version IS NULL) THEN
    INSERT INTO link (
      id,
      key,
      link_type_id,
      start_item_id,
      end_item_id,
      description,
      meta,
      tag,
      attribute,
      version,
      created,
      updated,
      changedby,
      transaction_ref
    )
    VALUES (
      nextval('link_id_seq'),
      key_param,
      link_type_id_value,
      start_item_id_value,
      end_item_id_value,
      description_param,
      meta_param,
      tag_param,
      attribute_param,
      1,
      current_timestamp,
      null,
      changedby_param,
      transaction_ref_param
    );
    result := 'I';
  ELSE
    UPDATE link SET
      meta = meta_param,
      description = description_param,
      tag = tag_param,
      attribute = attribute_param,
      link_type_id = link_type_id_value,
      start_item_id = start_item_id_value,
      end_item_id = end_item_id_value,
      version = version + 1,
      updated = current_timestamp,
      changedby = changedby_param,
      transaction_ref = transaction_ref_param
    WHERE key = key_param
    -- concurrency management - optimistic locking
    AND (local_version_param = current_version OR local_version_param IS NULL)
    AND (
      meta != meta_param OR
      description != description_param OR
      tag != tag_param OR
      attribute != attribute_param OR
      link_type_id != link_type_id_value OR
      start_item_id != start_item_id_value OR
      end_item_id != end_item_id_value OR
      transaction_ref != transaction_ref_param
    );
    GET DIAGNOSTICS rows_affected := ROW_COUNT;
    SELECT get_update_status(current_version, local_version_param, rows_affected > 0) INTO result;
  END IF;
  RETURN QUERY SELECT result;
END;
$BODY$;

ALTER FUNCTION set_link(character varying, character varying, character varying, character varying, text, jsonb, text[], hstore, bigint, character varying, uuid)
  OWNER TO onix;

/*
  set_link_rule(...)
  Inserts a new or updates an existing link rule.
  Concurrency Management:
   - If the link rule is found in the database, the function attempts an update of the existing record.
      In this case, if a null value is passed as local_version_param, no optimistic locking is performed.
      If a value is specified for local_version_param, the update is only performed if and only if the version in the database matches the passed in version.
   - If the link rule is not found in the database, then the local_version_param is ignored and a record with version 1 is inserted.

 */
CREATE OR REPLACE FUNCTION set_link_rule(
    key_param character varying,
    name_param character varying,
    description_param text,
    link_type_key_param character varying,
    start_item_type_key_param character varying,
    end_item_type_key_param character varying,
    local_version_param bigint,
    changedby_param character varying
  )
  RETURNS TABLE(result char(1))
  LANGUAGE 'plpgsql'
  COST 100
  VOLATILE
AS $BODY$
DECLARE
  result char(1); -- the result status for the upsert
  current_version bigint; -- the version of the row before the update or null if no row
  rows_affected integer;
  link_type_id_value integer;
  start_item_type_id_value integer;
  end_item_type_id_value integer;
BEGIN
  -- gets the current version
  SELECT version FROM link_rule WHERE key = key_param INTO current_version;

  -- gets the required id's
  SELECT id FROM link_type WHERE key = link_type_key_param INTO link_type_id_value;
  SELECT id FROM item_type WHERE key = start_item_type_key_param INTO start_item_type_id_value;
  SELECT id FROM item_type WHERE key = end_item_type_key_param INTO end_item_type_id_value;

  IF (current_version IS NULL) THEN
    INSERT INTO link_rule (
      id,
      key,
      name,
      description,
      link_type_id,
      start_item_type_id,
      end_item_type_id,
      version,
      created,
      updated,
      changedby
    )
    VALUES (
      nextval('link_rule_id_seq'),
      key_param,
      name_param,
      description_param,
      link_type_id_value,
      start_item_type_id_value,
      end_item_type_id_value,
      1,
      current_timestamp,
      null,
      changedby_param
    );
    result := 'I';
  ELSE
    UPDATE link_rule SET
       name = name_param,
       description = description_param,
       link_type_id = link_type_id_value,
       start_item_type_id = start_item_type_id_value,
       end_item_type_id = end_item_type_id_value,
       version = version + 1,
       updated = current_timestamp,
       changedby = changedby_param
    WHERE key = key_param
    -- concurrency management - optimistic locking (disabled if local_version_param is null)
    AND (local_version_param = current_version OR local_version_param IS NULL)
    AND (
      name != name_param OR
      description != description_param OR
      link_type_id != link_type_id_value OR
      start_item_type_id != start_item_type_id_value OR
      end_item_type_id != end_item_type_id_value
    );
    GET DIAGNOSTICS rows_affected := ROW_COUNT;
    SELECT get_update_status(current_version, local_version_param, rows_affected > 0) INTO result;
  END IF;
  RETURN QUERY SELECT result;
END;
$BODY$;

ALTER FUNCTION set_link_rule(character varying, character varying, text, character varying, character varying, character varying, bigint, character varying)
  OWNER TO onix;

END
$$;