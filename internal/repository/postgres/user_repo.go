package postgres

import (
	"context"

	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// UserRepo is the GORM implementation of repository.UserRepository.
type UserRepo struct {
	DB *gorm.DB
}

// NewUserRepo creates a new UserRepo.
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{DB: db}
}

func (r *UserRepo) FindByPhone(ctx context.Context, phone string) (*model.User, error) {
	var user model.User
	err := r.DB.WithContext(ctx).
		Preload("SchoolRoles.Role").Preload("SchoolRoles.School").
		Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	err := r.DB.WithContext(ctx).
		Preload("SchoolRoles.Role").Preload("SchoolRoles.School").
		First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByIDs(ctx context.Context, ids []uint, selectFields string) ([]model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var users []model.User
	query := r.DB.WithContext(ctx)
	if selectFields != "" {
		query = query.Select(selectFields)
	}
	err := query.Where("id IN ?", ids).Find(&users).Error
	return users, err
}

func (r *UserRepo) ListUsers(ctx context.Context, schoolID uint, offset, limit int) ([]model.User, int64, error) {
	query := r.DB.WithContext(ctx).
		Preload("SchoolRoles.Role").Preload("SchoolRoles.School")

	if schoolID > 0 {
		var userIDs []uint
		r.DB.WithContext(ctx).Model(&model.UserSchoolRole{}).
			Where("school_id = ?", schoolID).
			Pluck("user_id", &userIDs)
		if len(userIDs) == 0 {
			return nil, 0, nil
		}
		query = query.Where("id IN ?", userIDs)
	}

	var total int64
	query.Model(&model.User{}).Count(&total)

	var users []model.User
	err := query.Offset(offset).Limit(limit).Find(&users).Error
	return users, total, err
}

func (r *UserRepo) CreateUserWithRole(ctx context.Context, user *model.User, role *model.UserSchoolRole) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		role.UserID = user.ID
		return tx.Create(role).Error
	})
}

func (r *UserRepo) FindRoleByName(ctx context.Context, name model.RoleName) (*model.Role, error) {
	var role model.Role
	err := r.DB.WithContext(ctx).Where("name = ?", name).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *UserRepo) FindStudentClassIDs(ctx context.Context, studentID uint) ([]uint, error) {
	var classIDs []uint
	err := r.DB.WithContext(ctx).
		Raw("SELECT class_id FROM class_students WHERE user_id = ?", studentID).
		Scan(&classIDs).Error
	return classIDs, err
}

func (r *UserRepo) FindStudentIDsByClassID(ctx context.Context, classID uint) ([]uint, error) {
	var studentIDs []uint
	err := r.DB.WithContext(ctx).Model(&model.ClassStudent{}).
		Where("class_id = ?", classID).
		Pluck("student_id", &studentIDs).Error
	return studentIDs, err
}

func (r *UserRepo) ListSchools(ctx context.Context, offset, limit int) ([]model.School, int64, error) {
	var total int64
	r.DB.WithContext(ctx).Model(&model.School{}).Count(&total)

	var schools []model.School
	err := r.DB.WithContext(ctx).Offset(offset).Limit(limit).Find(&schools).Error
	return schools, total, err
}

func (r *UserRepo) CreateSchool(ctx context.Context, school *model.School) error {
	return r.DB.WithContext(ctx).Create(school).Error
}

func (r *UserRepo) ListClasses(ctx context.Context, schoolID uint, offset, limit int) ([]model.Class, int64, error) {
	query := r.DB.WithContext(ctx).Preload("School")
	if schoolID > 0 {
		query = query.Where("school_id = ?", schoolID)
	}

	var total int64
	query.Model(&model.Class{}).Count(&total)

	var classes []model.Class
	err := query.Offset(offset).Limit(limit).Find(&classes).Error
	return classes, total, err
}

func (r *UserRepo) CreateClass(ctx context.Context, class *model.Class) error {
	return r.DB.WithContext(ctx).Create(class).Error
}
