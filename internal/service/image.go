package service

import (
	"HorizonBackend/internal/model"
	"HorizonBackend/internal/repository/postgres"
	"errors"
	"log"
)

type ImageService interface {
	GetImagesByFamilyGroupSubgroup(family, group, subgroup string) ([]model.Image, error)
	SearchImages(keyword, family string) ([]model.Image, error)
	GetImageByNumber(family, group, subgroup, imageNumber string) (*model.Image, error)
	IncreaseUsageCount(thumbPath string) error
	GetLeastUsedImages(family string, limit int) ([]model.Image, error)
}

type imageServiceImpl struct {
	repo *postgres.ImageRepository
}

func NewImageService(repo *postgres.ImageRepository) ImageService {
	return &imageServiceImpl{repo: repo}
}

func (s *imageServiceImpl) GetImagesByFamilyGroupSubgroup(family, group, subgroup string) ([]model.Image, error) {
	// Валидация
	if family == "" || group == "" {
		log.Println("Invalid input: family, group or subgroup is empty")
		return nil, errors.New("family, group or subgroup cannot be empty")
	}

	// Получение изображений
	images, err := s.repo.GetImagesByFamilyGroupSubgroup(family, group, subgroup)
	if err != nil {
		log.Printf("Service error fetching images for family: %s, group: %s and subgroup: %s Error: %v", family, group, subgroup, err)
		return nil, err
	}

	for i := range images {
		images[i].UsageCount++
	}

	return images, nil
}

func (s *imageServiceImpl) SearchImages(keyword, family string) ([]model.Image, error) {
	return s.repo.SearchImagesByKeywordAndFamily(keyword, family)
}

func (s *imageServiceImpl) GetImageByNumber(family, group, subgroup, imageNumber string) (*model.Image, error) {
	image, err := s.repo.FindImageByNumber(family, group, subgroup, imageNumber)
	if err != nil {
		log.Printf("Service error fetching image by number for family: %s, group: %s, subgroup: %s, number: %s, Error: %v", family, group, subgroup, imageNumber, err)
		return nil, err
	}
	return image, nil
}

func (s *imageServiceImpl) IncreaseUsageCount(thumbPath string) error {
	return s.repo.IncreaseUsageCount(thumbPath)
}

func (s *imageServiceImpl) GetLeastUsedImages(family string, limit int) ([]model.Image, error) {
	return s.repo.GetLeastUsedImages(family, limit)
}
