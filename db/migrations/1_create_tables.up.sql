CREATE TABLE Families (
                          id SERIAL PRIMARY KEY,
                          name TEXT NOT NULL UNIQUE
);

CREATE TABLE Groups (
                        id SERIAL PRIMARY KEY,
                        family_id INTEGER REFERENCES Families(id),
                        name TEXT NOT NULL,
                        UNIQUE(family_id, name)
);

-- Добавляем таблицу подгрупп
CREATE TABLE Subgroups (
                           id SERIAL PRIMARY KEY,
                           group_id INTEGER REFERENCES Groups(id),
                           name TEXT NOT NULL,
                           UNIQUE(group_id, name)
);

CREATE TABLE Images (
                        id SERIAL PRIMARY KEY,
                        subgroup_id INTEGER REFERENCES Subgroups(id), -- теперь изображения относятся к подгруппам
                        name TEXT NOT NULL,
                        file_path TEXT,
                        thumb_path TEXT,
                        usage_count INTEGER DEFAULT 0,
                        meta_tags TEXT[],
                        UNIQUE(name, subgroup_id)
);

-- CREATE INDEX idx_images_name_group ON Images (name, subgroup_id);
