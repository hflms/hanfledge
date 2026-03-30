//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/repository/postgres"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// WeKnoraUser represents a user in WeKnora database
type WeKnoraUser struct {
	ID                  string `gorm:"primaryKey;type:varchar(36);default:uuid_generate_v4()"`
	Username            string `gorm:"uniqueIndex;not null"`
	Email               string `gorm:"uniqueIndex;not null"`
	PasswordHash        string `gorm:"not null"`
	TenantID            *int
	IsActive            bool `gorm:"default:true"`
	CanAccessAllTenants bool `gorm:"default:false"`
}

func (WeKnoraUser) TableName() string {
	return "users"
}

func main() {
	cfg := config.Load()

	// Connect to Hanfledge DB
	hanfledgeDB, err := postgres.NewConnection(&cfg.Database)
	if err != nil {
		log.Fatal("failed to connect to hanfledge db:", err)
	}

	// Connect to WeKnora DB
	wkCfg := cfg.Database
	wkCfg.DBName = "weknora"
	wkDB, err := postgres.NewConnection(&wkCfg)
	if err != nil {
		log.Fatal("failed to connect to weknora db:", err)
	}

	// Get all Hanfledge users with admin roles
	var users []model.User
	if err := hanfledgeDB.Preload("SchoolRoles.Role").Find(&users).Error; err != nil {
		log.Fatal("failed to fetch users:", err)
	}

	ctx := context.Background()
	synced := 0

	for _, user := range users {
		// Check if user has admin role
		isAdmin := false
		for _, schoolRole := range user.SchoolRoles {
			if schoolRole.Role.Name == model.RoleSysAdmin || schoolRole.Role.Name == model.RoleSchoolAdmin {
				isAdmin = true
				break
			}
		}

		// Sync user to WeKnora
		// Use phone as plaintext password for WeKnora
		plaintextPassword := user.Phone
		hashedPassword, hashErr := hashPassword(plaintextPassword)
		if hashErr != nil {
			log.Printf("failed to hash password for %s: %v", user.Phone, hashErr)
			continue
		}

		wkUser := WeKnoraUser{
			ID:                  uuid.New().String(),
			Username:            user.Phone, // Use phone as username
			Email:               fmt.Sprintf("%s@hanfledge.local", user.Phone),
			PasswordHash:        hashedPassword, // Hash the phone number
			IsActive:            true,
			CanAccessAllTenants: isAdmin, // Admin users can access all tenants
		}

		// Check if user already exists
		var existing WeKnoraUser
		err := wkDB.WithContext(ctx).Where("username = ?", wkUser.Username).First(&existing).Error
		if err == nil {
			// Update existing user
			if err := wkDB.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
				"email":                  wkUser.Email,
				"password_hash":          wkUser.PasswordHash,
				"can_access_all_tenants": wkUser.CanAccessAllTenants,
			}).Error; err != nil {
				log.Printf("failed to update user %s: %v", user.Phone, err)
				continue
			}
			log.Printf("✓ Updated WeKnora user: %s (admin=%v)", user.Phone, isAdmin)
		} else if err == gorm.ErrRecordNotFound {
			// Create new user
			if err := wkDB.WithContext(ctx).Create(&wkUser).Error; err != nil {
				log.Printf("failed to create user %s: %v", user.Phone, err)
				continue
			}
			log.Printf("✓ Created WeKnora user: %s (admin=%v)", user.Phone, isAdmin)
		} else {
			log.Printf("failed to check user %s: %v", user.Phone, err)
			continue
		}

		synced++
	}

	log.Printf("\n✅ Synced %d users to WeKnora", synced)
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
