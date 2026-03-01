package main

import (
	"fmt"

	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/hflms/hanfledge/internal/repository/postgres"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// seed.go 填充测试数据：1 管理员, 1 学校, 2 班级, 2 教师, 10 学生
// Usage: go run scripts/seed.go

func main() {
	logger.Init("debug")
	log := logger.L("Seed")

	cfg := config.Load()
	db, err := postgres.NewConnection(&cfg.Database)
	if err != nil {
		logger.Fatal("db connection failed", "err", err)
	}

	// Ensure tables exist
	if err := postgres.AutoMigrate(db); err != nil {
		logger.Fatal("migration failed", "err", err)
	}

	log.Info("seeding test data")

	// ── 1. Create SYS_ADMIN ──────────────────────────────
	admin := createUser(db, "13800000001", "Admin", "admin123")
	assignRole(db, admin.ID, nil, model.RoleSysAdmin)
	log.Info("created sys_admin", "name", admin.DisplayName, "phone", admin.Phone)

	// ── 2. Create School ─────────────────────────────────
	school := model.School{
		Name:   "杭州示范中学",
		Code:   "HZSF001",
		Region: "浙江省杭州市",
	}
	db.FirstOrCreate(&school, model.School{Code: school.Code})
	log.Info("created school", "name", school.Name, "id", school.ID)

	// ── 3. Create Classes ────────────────────────────────
	class1 := model.Class{SchoolID: school.ID, Name: "高一(1)班", GradeLevel: 10, AcademicYear: "2025-2026"}
	class2 := model.Class{SchoolID: school.ID, Name: "高一(2)班", GradeLevel: 10, AcademicYear: "2025-2026"}
	db.FirstOrCreate(&class1, model.Class{SchoolID: school.ID, Name: class1.Name})
	db.FirstOrCreate(&class2, model.Class{SchoolID: school.ID, Name: class2.Name})
	log.Info("created classes", "class1", class1.Name, "class2", class2.Name)

	// ── 4. Create Teachers ───────────────────────────────
	teacher1 := createUser(db, "13800000010", "张数学老师", "teacher123")
	assignRole(db, teacher1.ID, &school.ID, model.RoleTeacher)
	// 张老师同时担任学校管理员
	assignRole(db, teacher1.ID, &school.ID, model.RoleSchoolAdmin)

	teacher2 := createUser(db, "13800000011", "李物理老师", "teacher123")
	assignRole(db, teacher2.ID, &school.ID, model.RoleTeacher)

	log.Info("created teachers", "teacher1", teacher1.DisplayName, "roles1", "TEACHER+SCHOOL_ADMIN", "teacher2", teacher2.DisplayName, "roles2", "TEACHER")

	// ── 5. Create Students ───────────────────────────────
	studentNames := []string{"王小明", "赵小红", "刘小刚", "陈小美", "杨小亮", "周小华", "黄小军", "吴小芳", "郑小龙", "孙小丽"}
	for i, name := range studentNames {
		phone := fmt.Sprintf("1380000%04d", 100+i)
		student := createUser(db, phone, name, "student123")
		assignRole(db, student.ID, &school.ID, model.RoleStudent)

		// Assign to classes: first 5 to class1, rest to class2
		classID := class1.ID
		if i >= 5 {
			classID = class2.ID
		}
		db.FirstOrCreate(&model.ClassStudent{
			ClassID:   classID,
			StudentID: student.ID,
		}, model.ClassStudent{ClassID: classID, StudentID: student.ID})
	}
	log.Info("created students", "count", len(studentNames), "per_class", 5)

	log.Info("seed data complete")
	log.Info("test account", "role", "admin", "username", "13800000001", "password", "admin123")
	log.Info("test account", "role", "teacher", "username", "13800000010", "password", "teacher123", "name", "张数学老师", "extra_role", "SCHOOL_ADMIN")
	log.Info("test account", "role", "teacher", "username", "13800000011", "password", "teacher123", "name", "李物理老师")
	log.Info("test account", "role", "student", "username", "13800000100", "password", "student123", "name", "王小明", "class", "高一1班")
	log.Info("test account", "role", "student", "username", "13800000105", "password", "student123", "name", "周小华", "class", "高一2班")
}

// createUser creates a user if not exists (by phone), returns the user.
func createUser(db *gorm.DB, phone, name, password string) model.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := model.User{
		Phone:        phone,
		PasswordHash: string(hash),
		DisplayName:  name,
		Status:       model.UserStatusActive,
	}
	db.FirstOrCreate(&user, model.User{Phone: phone})
	return user
}

// assignRole assigns a role to a user in a school.
func assignRole(db *gorm.DB, userID uint, schoolID *uint, roleName model.RoleName) {
	var role model.Role
	db.Where("name = ?", roleName).First(&role)

	usr := model.UserSchoolRole{
		UserID:   userID,
		SchoolID: schoolID,
		RoleID:   role.ID,
	}
	db.FirstOrCreate(&usr, model.UserSchoolRole{
		UserID:   userID,
		SchoolID: schoolID,
		RoleID:   role.ID,
	})
}
