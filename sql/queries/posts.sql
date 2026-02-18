-- name: CreatePost :one
INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES (
           $1,
           $2,
           $3,
           $4,
           $5,
           $6,
           $7,
           $8
       )
    RETURNING *;

-- name: GetPostsForUser :many
SELECT posts.id,
       posts.created_at,
       posts.updated_at,
       posts.title,
       posts.url,
       posts.description,
       posts.published_at,
       posts.feed_id,
       feeds.name AS feed_name
FROM posts
INNER JOIN feeds ON posts.feed_id = feeds.id
INNER JOIN feed_follows ON feed_follows.feed_id = feeds.id
WHERE feed_follows.user_id = $1
  AND ($2::text = '' OR posts.title ILIKE '%' || $2 || '%' OR COALESCE(posts.description, '') ILIKE '%' || $2 || '%')
  AND ($3::text = '' OR feeds.name ILIKE '%' || $3 || '%')
ORDER BY posts.published_at DESC NULLS LAST, posts.created_at DESC
LIMIT $4;
