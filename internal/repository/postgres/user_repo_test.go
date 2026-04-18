package postgres

import (
	"context"
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupLocalTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&model.User{},
		&model.School{},
		&model.Class{},
		&model.Role{},
		&model.UserSchoolRole{},
		&model.ClassStudent{},
	)
	require.NoError(t, err)

	return db
}

func TestNewUserRepo(t *testing.T) {
	db := setupLocalTestDB(t)
	repo := NewUserRepo(db)
	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.DB)
}

func TestUserRepo_SchoolOperations(t *testing.T) {
	db := setupLocalTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	// ListSchools when empty
	schools, total, err := repo.ListSchools(ctx, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, schools)
	assert.Equal(t, int64(0), total)

	// CreateSchool
	school1 := &model.School{Name: "School 1", Code: "S1"}
	err = repo.CreateSchool(ctx, school1)
	require.NoError(t, err)
	assert.NotZero(t, school1.ID)

	school2 := &model.School{Name: "School 2", Code: "S2"}
	err = repo.CreateSchool(ctx, school2)
	require.NoError(t, err)

	// ListSchools after creation
	schools, total, err = repo.ListSchools(ctx, 0, 10)
	require.NoError(t, err)
	assert.Len(t, schools, 2)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, "School 1", schools[0].Name)
	assert.Equal(t, "School 2", schools[1].Name)

	// Pagination
	schools, total, err = repo.ListSchools(ctx, 1, 1)
	require.NoError(t, err)
	assert.Len(t, schools, 1)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, "School 2", schools[0].Name)
}

func TestUserRepo_ClassOperations(t *testing.T) {
	db := setupLocalTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	school := &model.School{Name: "School 1", Code: "S1"}
	require.NoError(t, repo.CreateSchool(ctx, school))

	// ListClasses when empty
	classes, total, err := repo.ListClasses(ctx, school.ID, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, classes)
	assert.Equal(t, int64(0), total)

	// CreateClass
	class1 := &model.Class{Name: "Class 1", SchoolID: school.ID, GradeLevel: 1}
	err = repo.CreateClass(ctx, class1)
	require.NoError(t, err)
	assert.NotZero(t, class1.ID)

	class2 := &model.Class{Name: "Class 2", SchoolID: school.ID, GradeLevel: 1}
	err = repo.CreateClass(ctx, class2)
	require.NoError(t, err)

	// Another school's class
	school2 := &model.School{Name: "School 2", Code: "S2"}
	require.NoError(t, repo.CreateSchool(ctx, school2))
	class3 := &model.Class{Name: "Class 3", SchoolID: school2.ID, GradeLevel: 2}
	require.NoError(t, repo.CreateClass(ctx, class3))

	// ListClasses for school 1
	classes, total, err = repo.ListClasses(ctx, school.ID, 0, 10)
	require.NoError(t, err)
	assert.Len(t, classes, 2)
	assert.Equal(t, int64(2), total)

	// ListClasses for all
	classesAll, totalAll, err := repo.ListClasses(ctx, 0, 0, 10)
	require.NoError(t, err)
	assert.Len(t, classesAll, 3)
	assert.Equal(t, int64(3), totalAll)
}

func TestUserRepo_UserOperations(t *testing.T) {
	db := setupLocalTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	school := &model.School{Name: "School 1", Code: "S1"}
	require.NoError(t, repo.CreateSchool(ctx, school))

	role := &model.Role{Name: model.RoleStudent}
	require.NoError(t, db.Create(role).Error)

	// CreateUserWithRole
	user1 := &model.User{Phone: "1234567890", DisplayName: "User 1", PasswordHash: "hash"}
	userRole1 := &model.UserSchoolRole{SchoolID: &school.ID, RoleID: role.ID}
	err := repo.CreateUserWithRole(ctx, user1, userRole1)
	require.NoError(t, err)
	assert.NotZero(t, user1.ID)
	assert.NotZero(t, userRole1.ID)
	assert.Equal(t, user1.ID, userRole1.UserID)

	// FindByID
	foundUser, err := repo.FindByID(ctx, user1.ID)
	require.NoError(t, err)
	assert.Equal(t, user1.Phone, foundUser.Phone)
	assert.Len(t, foundUser.SchoolRoles, 1)

	// FindByID - Not Found
	_, err = repo.FindByID(ctx, 999)
	assert.Error(t, err)

	// FindByPhone
	foundUserPhone, err := repo.FindByPhone(ctx, "1234567890")
	require.NoError(t, err)
	assert.Equal(t, user1.ID, foundUserPhone.ID)

	// FindByPhone - Not Found
	_, err = repo.FindByPhone(ctx, "000")
	assert.Error(t, err)

	// FindByIDs
	user2 := &model.User{Phone: "0987654321", DisplayName: "User 2", PasswordHash: "hash"}
	userRole2 := &model.UserSchoolRole{SchoolID: &school.ID, RoleID: role.ID}
	require.NoError(t, repo.CreateUserWithRole(ctx, user2, userRole2))

	users, err := repo.FindByIDs(ctx, []uint{user1.ID, user2.ID}, "id, phone")
	require.NoError(t, err)
	assert.Len(t, users, 2)

	// FindByIDs with empty slice
	emptyUsers, err := repo.FindByIDs(ctx, []uint{}, "")
	require.NoError(t, err)
	assert.Empty(t, emptyUsers)

	// ListUsers
	listedUsers, total, err := repo.ListUsers(ctx, school.ID, 0, 10)
	require.NoError(t, err)
	assert.Len(t, listedUsers, 2)
	assert.Equal(t, int64(2), total)

	// ListUsers - All schools
	allUsers, allTotal, err := repo.ListUsers(ctx, 0, 0, 10)
	require.NoError(t, err)
	assert.Len(t, allUsers, 2)
	assert.Equal(t, int64(2), allTotal)

	// ListUsers - Empty School
	school2 := &model.School{Name: "School 2", Code: "S2"}
	require.NoError(t, repo.CreateSchool(ctx, school2))
	emptySchoolUsers, emptyTotal, err := repo.ListUsers(ctx, school2.ID, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, emptySchoolUsers)
	assert.Equal(t, int64(0), emptyTotal)
}

func TestUserRepo_FindRoleByName(t *testing.T) {
	db := setupLocalTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	role := &model.Role{Name: model.RoleTeacher}
	require.NoError(t, db.Create(role).Error)

	foundRole, err := repo.FindRoleByName(ctx, model.RoleTeacher)
	require.NoError(t, err)
	assert.Equal(t, role.ID, foundRole.ID)

	_, err = repo.FindRoleByName(ctx, model.RoleStudent)
	assert.Error(t, err)
}

func TestUserRepo_ClassStudentOperations(t *testing.T) {
	db := setupLocalTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	school := &model.School{Name: "School", Code: "S"}
	require.NoError(t, repo.CreateSchool(ctx, school))

	class1 := &model.Class{Name: "C1", SchoolID: school.ID, GradeLevel: 1}
	class2 := &model.Class{Name: "C2", SchoolID: school.ID, GradeLevel: 1}
	require.NoError(t, repo.CreateClass(ctx, class1))
	require.NoError(t, repo.CreateClass(ctx, class2))

	student1 := &model.User{Phone: "1", DisplayName: "U1", PasswordHash: "h"}
	student2 := &model.User{Phone: "2", DisplayName: "U2", PasswordHash: "h"}
	require.NoError(t, db.Create(student1).Error)
	require.NoError(t, db.Create(student2).Error)

	require.NoError(t, db.Create(&model.ClassStudent{ClassID: class1.ID, StudentID: student1.ID}).Error)
	require.NoError(t, db.Create(&model.ClassStudent{ClassID: class2.ID, StudentID: student1.ID}).Error)
	require.NoError(t, db.Create(&model.ClassStudent{ClassID: class1.ID, StudentID: student2.ID}).Error)

	// FindStudentClassIDs
	classIDs, err := repo.FindStudentClassIDs(ctx, student1.ID)
	require.NoError(t, err)
	assert.Len(t, classIDs, 2)
	assert.Contains(t, classIDs, class1.ID)
	assert.Contains(t, classIDs, class2.ID)

	// FindStudentIDsByClassID
	studentIDs, err := repo.FindStudentIDsByClassID(ctx, class1.ID)
	require.NoError(t, err)
	assert.Len(t, studentIDs, 2)
	assert.Contains(t, studentIDs, student1.ID)
	assert.Contains(t, studentIDs, student2.ID)
}
