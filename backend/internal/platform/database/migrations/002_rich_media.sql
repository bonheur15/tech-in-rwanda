ALTER TABLE media_assets ADD COLUMN crop_aspect TEXT NOT NULL DEFAULT 'original' CHECK(crop_aspect IN ('original','16:9','4:3','1:1'));

CREATE TABLE article_media_new (
  post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  asset_id TEXT NOT NULL REFERENCES media_assets(id),
  placement TEXT NOT NULL CHECK(placement IN ('thumbnail','small','center','wide','full','left','right')),
  PRIMARY KEY(post_id,asset_id,placement)
);
INSERT INTO article_media_new SELECT post_id,asset_id,placement FROM article_media;
DROP TABLE article_media;
ALTER TABLE article_media_new RENAME TO article_media;
