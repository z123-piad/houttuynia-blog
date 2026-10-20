-- name: CreateAsset :one
insert into assets (storage, bucket, object_key, mime, size, owner_id, visibility, checksum)
values ($1, $2, $3, $4, $5, $6, $7, $8)
returning id, storage, bucket, object_key, mime, size, owner_id, visibility, checksum, deleted_at;

-- name: GetAssetByID :one
select id, storage, bucket, object_key, mime, size, owner_id, visibility, checksum, deleted_at
from assets
where id = $1 and deleted_at is null;

-- name: GetLiveAssetByKey :one
select id, storage, bucket, object_key, mime, size, owner_id, visibility, checksum, deleted_at
from assets
where storage = $1 and object_key = $2 and deleted_at is null;

-- name: SoftDeleteAsset :execrows
update assets
set deleted_at = now(), updated_at = now()
where id = $1 and deleted_at is null;
