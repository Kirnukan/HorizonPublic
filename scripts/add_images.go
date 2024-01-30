package scripts

import (
	"database/sql"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"
)

func compressImage(inputPath string, outputPath string) error {
	fmt.Println("Processing:", inputPath)
	file, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return err
	}

	if format != "jpeg" && format != "png" {
		return fmt.Errorf("unsupported format for file: %s", inputPath)
	}

	m := resize.Resize(100, 100, img, resize.Lanczos3)

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if filepath.Ext(inputPath) == ".jpg" || filepath.Ext(inputPath) == ".jpeg" {
		err = jpeg.Encode(out, m, nil)
	} else if filepath.Ext(inputPath) == ".png" {
		err = png.Encode(out, m)
	}

	return err
}

func toInterfaceSlice(slice []int) []interface{} {
	s := make([]interface{}, len(slice))
	for i, v := range slice {
		s[i] = v
	}
	return s
}

func getFilePathFromDB(db *sql.DB, imageID int) string {
	var filePath string

	// запрос SQL для извлечения пути файла по ID
	query := `SELECT file_path FROM images WHERE id = $1`

	err := db.QueryRow(query, imageID).Scan(&filePath)
	if err != nil {
		log.Printf("Error retrieving file path for ID %d: %v\n", imageID, err)
		return ""
	}

	return filePath
}

func AddImagesFromFolder(db *sql.DB, baseFolder string) {

	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	fmt.Println("Step 1: Checking for file existence.")
	rows, err := db.Query(`SELECT id, file_path FROM Images`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var idsToDelete []int
	for rows.Next() {
		var id int
		var filePath string
		if err := rows.Scan(&id, &filePath); err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			continue
		}

		absolutePath := filePath
		if _, err := os.Stat(absolutePath); os.IsNotExist(err) {
			fmt.Printf("Marking ID: %d, FilePath: %s for deletion. AbsolutePath: %s. Error: %v\n",
				id, filePath, absolutePath, err)
			idsToDelete = append(idsToDelete, id)
		} else if err != nil {
			fmt.Printf("Error checking file existence, ID: %d, FilePath: %s, AbsolutePath: %s. Error: %v\n",
				id, filePath, absolutePath, err)
		}
	}
	for _, idToDelete := range idsToDelete {
		filePath := getFilePathFromDB(db, idToDelete)
		fmt.Printf("Deleting entry with ID: %d, FilePath: %s\n", idToDelete, filePath)

	}

	// Удаляем записи, которых нет на диске

	if len(idsToDelete) > 0 {
		fmt.Printf("Deleting %d Image entries as files do not exist on disk.\n", len(idsToDelete))

		idsStr := ""
		for i := range idsToDelete {
			idsStr += fmt.Sprintf("$%d,", i+1)
		}
		idsStr = strings.TrimSuffix(idsStr, ",")

		query := "DELETE FROM Images WHERE id IN (" + idsStr + ")"
		_, err = db.Exec(query, toInterfaceSlice(idsToDelete)...)
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Println("No Image entries need deletion.")
	}

	fmt.Println("Step 2: Adding new files.")

	familyDirs, err := os.ReadDir(baseFolder)
	if err != nil {
		panic(err)
	}

	for _, familyDir := range familyDirs {
		fmt.Printf("Processing family: %s\n", familyDir.Name())
		if !familyDir.IsDir() {
			continue
		}
		familyName := familyDir.Name()

		_, err := tx.Exec(`INSERT INTO Families (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`, familyName)
		if err != nil {
			panic(err)
		}

		groupDirs, err := os.ReadDir(filepath.Join(baseFolder, familyName))
		if err != nil {
			panic(err)
		}

		for _, groupDir := range groupDirs {
			fmt.Printf("Processing group: %s\n", groupDir.Name())
			if !groupDir.IsDir() {
				continue
			}
			groupName := groupDir.Name()

			_, err := tx.Exec(`
                INSERT INTO Groups (name, family_id) 
                VALUES ($1, (SELECT id FROM Families WHERE name = $2)) 
                ON CONFLICT (family_id, name) DO NOTHING`, groupName, familyName)
			if err != nil {
				panic(err)
			}

			subgroupDirs, err := os.ReadDir(filepath.Join(baseFolder, familyName, groupName))
			if err != nil {
				panic(err)
			}

			for _, subgroupDir := range subgroupDirs {
				fmt.Printf("Processing subgroup: %s\n", subgroupDir.Name())

				if !subgroupDir.IsDir() {
					continue
				}
				subgroupName := subgroupDir.Name()

				_, err := tx.Exec(`
                    INSERT INTO Subgroups (name, group_id) 
                    VALUES ($1, (SELECT id FROM Groups WHERE name = $2 AND family_id = (SELECT id FROM Families WHERE name = $3)))
                    ON CONFLICT (group_id, name) DO NOTHING`, subgroupName, groupName, familyName)
				if err != nil {
					panic(err)
				}

				imageFiles, err := os.ReadDir(filepath.Join(baseFolder, familyName, groupName, subgroupName))
				if err != nil {
					panic(err)
				}

				for _, imageFile := range imageFiles {
					fmt.Printf("Processing image file: %s\n", imageFile.Name())

					if imageFile.IsDir() {
						continue
					}
					imageName := strings.TrimSuffix(imageFile.Name(), filepath.Ext(imageFile.Name()))

					if strings.Contains(imageName, "_thumb") {
						continue
					}

					imagePath := filepath.Join("static", "images", familyName, groupName, subgroupName, imageFile.Name())
					thumbPath := ""

					if familyName == "Frames" {
						thumbPath = imagePath
					} else {
						thumbPath = filepath.Join("static", "images", familyName, groupName, subgroupName, imageName+"_thumb"+filepath.Ext(imageFile.Name()))
						originalFilePath := filepath.Join(baseFolder, familyName, groupName, subgroupName, imageFile.Name())
						thumbFilePath := filepath.Join(baseFolder, familyName, groupName, subgroupName, imageName+"_thumb"+filepath.Ext(imageFile.Name()))
						if _, err := os.Stat(thumbFilePath); os.IsNotExist(err) {
							err = compressImage(originalFilePath, thumbFilePath)
							if err != nil {
								panic(err)
							}
						}
					}

					_, err := tx.Exec(`
						INSERT INTO Images (name, file_path, thumb_path, subgroup_id)
						VALUES ($1, $2, $3, (SELECT s.id FROM Subgroups s
											 JOIN Groups g ON s.group_id = g.id
											 WHERE s.name = $4 AND g.name = $5 AND g.family_id = (SELECT id FROM Families WHERE name = $6) LIMIT 1))
						ON CONFLICT (name, subgroup_id)
						DO UPDATE SET file_path = excluded.file_path, thumb_path = excluded.thumb_path`,
						imageName, imagePath, thumbPath, subgroupName, groupName, familyName)
					if err != nil {
						fmt.Printf("Error inserting/updating image: %s\n", err.Error())
						panic(err)
					} else {
						fmt.Printf("Image [%s] processed successfully.\n", imageName)
					}

				}
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		panic(err)
	}
}
