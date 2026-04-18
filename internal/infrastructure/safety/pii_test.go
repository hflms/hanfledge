package safety

import (
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupPIITestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}

	err = db.AutoMigrate(
		&model.User{},
		&model.School{},
		&model.Role{},
		&model.UserSchoolRole{},
	)
	if err != nil {
		t.Fatalf("failed to auto migrate: %v", err)
	}

	return db
}

func seedPIITestData(t *testing.T, db *gorm.DB) {
	t.Helper()
	studentRole := model.Role{Name: model.RoleStudent}
	teacherRole := model.Role{Name: model.RoleTeacher}
	sysAdminRole := model.Role{Name: model.RoleSysAdmin}
	db.Create(&studentRole)
	db.Create(&teacherRole)
	db.Create(&sysAdminRole)

	school1 := model.School{Name: "Test School Academy", Code: "TSA"}
	school2 := model.School{Name: "Second High School", Code: "SHS"}
	db.Create(&school1)
	db.Create(&school2)

	student1 := model.User{DisplayName: "Alice", Phone: "13800138000"}
	student2 := model.User{DisplayName: "Bob", Phone: "13900139000"}
	// Short name should be ignored
	student3 := model.User{DisplayName: "A", Phone: "13100131000"}
	db.Create(&student1)
	db.Create(&student2)
	db.Create(&student3)
	db.Create(&model.UserSchoolRole{UserID: student1.ID, SchoolID: &school1.ID, RoleID: studentRole.ID})
	db.Create(&model.UserSchoolRole{UserID: student2.ID, SchoolID: &school2.ID, RoleID: studentRole.ID})
	db.Create(&model.UserSchoolRole{UserID: student3.ID, SchoolID: &school1.ID, RoleID: studentRole.ID})

	teacher1 := model.User{DisplayName: "Mr. Smith", Phone: "13700137000"}
	teacher2 := model.User{DisplayName: "Mrs. Jones", Phone: "13600136000"}
	db.Create(&teacher1)
	db.Create(&teacher2)
	db.Create(&model.UserSchoolRole{UserID: teacher1.ID, SchoolID: &school1.ID, RoleID: teacherRole.ID})
	db.Create(&model.UserSchoolRole{UserID: teacher2.ID, SchoolID: &school2.ID, RoleID: teacherRole.ID})

    // Add deleted user which should be ignored
	deletedUser := model.User{DisplayName: "DeletedUser", Phone: "13500135000"}
    db.Create(&deletedUser)
    db.Create(&model.UserSchoolRole{UserID: deletedUser.ID, SchoolID: &school1.ID, RoleID: studentRole.ID})
    db.Delete(&deletedUser)
}

func TestPIIRedactor_Redact(t *testing.T) {
	db := setupPIITestDB(t)
	seedPIITestData(t, db)

	redactor := NewPIIRedactor(db)

	tests := []struct {
		name          string
		input         string
		expectedText  string
		expectedCount int
	}{
		{
			name:          "empty string",
			input:         "",
			expectedText:  "",
			expectedCount: 0,
		},
		{
			name:          "no PII",
			input:         "Hello world, this is a normal text.",
			expectedText:  "Hello world, this is a normal text.",
			expectedCount: 0,
		},
		{
			name:          "student name",
			input:         "Alice is a good student.",
			expectedText:  "[学生] is a good student.",
			expectedCount: 1,
		},
		{
			name:          "student name too short",
			input:         "A is a good student.",
			expectedText:  "A is a good student.", // Ignored because len < 2
			expectedCount: 0,
		},
		{
			name:          "teacher name",
			input:         "Mr. Smith teaches math.",
			expectedText:  "[教师] teaches math.",
			expectedCount: 1,
		},
		{
			name:          "school name",
			input:         "Welcome to Test School Academy!",
			expectedText:  "Welcome to [学校]!",
			expectedCount: 1,
		},
		{
			name:          "phone number",
			input:         "Call me at 13800138000.",
			expectedText:  "Call me at [手机号].",
			expectedCount: 1,
		},
		{
			name:          "email address",
			input:         "My email is test@example.com.",
			expectedText:  "My email is [邮箱].",
			expectedCount: 1,
		},
		{
			name:          "id card number",
			input:         "My ID is 110105199001011234.",
			expectedText:  "My ID is [证件号].",
			expectedCount: 1,
		},
		{
			name:          "multiple PII types",
			input:         "Alice goes to Test School Academy with Mr. Smith. Her phone is 13800138000, email alice@test.com, ID 110105199001011234.",
			expectedText:  "[学生] goes to [学校] with [教师]. Her phone is [手机号], email [邮箱], ID [证件号].",
			expectedCount: 6,
		},
        {
            name:          "deleted user should not be redacted",
            input:         "DeletedUser is not redacted.",
            expectedText:  "DeletedUser is not redacted.",
            expectedCount: 0,
        },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, count := redactor.Redact(tt.input)
			assert.Equal(t, tt.expectedText, result)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

func TestPIIRedactor_RedactMessagesIntegration(t *testing.T) {
	db := setupPIITestDB(t)
	seedPIITestData(t, db)

	redactor := NewPIIRedactor(db)

	messages := []ChatMessageLike{
		SimpleChatMessage{Role: "system", Content: "You are a helpful assistant. Alice is a student."},
		SimpleChatMessage{Role: "user", Content: "Hello, my name is Alice, my phone is 13800138000."},
		SimpleChatMessage{Role: "assistant", Content: "Hello Alice, I have noted your phone number 13800138000."},
	}

	redactedMessages, totalCount := redactor.RedactMessages(messages)

	assert.Equal(t, 2, totalCount)
	assert.Len(t, redactedMessages, 3)

	// System message should not be redacted
	assert.Equal(t, "You are a helpful assistant. Alice is a student.", redactedMessages[0].GetContent())

	// User message should be redacted
	assert.Equal(t, "Hello, my name is [学生], my phone is [手机号].", redactedMessages[1].GetContent())

	// Assistant message should not be redacted
	assert.Equal(t, "Hello Alice, I have noted your phone number 13800138000.", redactedMessages[2].GetContent())
}

func TestPIIRedactor_RedactForLog(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short text no phone",
			input:    "Hello world",
			maxLen:   50,
			expected: `"Hello world"`,
		},
		{
			name:     "truncate text",
			input:    "This is a very long text that should be truncated.",
			maxLen:   10,
			expected: `"This is a ..."`,
		},
		{
			name:     "redact phone",
			input:    "User phone is 13812345678",
			maxLen:   50,
			expected: `"User phone is 138****5678"`,
		},
		{
			name:     "truncate and redact phone",
			input:    "Phone 13812345678 is here",
			maxLen:   17,
			expected: `"Phone 138****5678..."`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactForLog(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPIIRedactor_sortByLengthDesc(t *testing.T) {
	strs := []string{"a", "abc", "ab", "abcd"}
	sortByLengthDesc(strs)
	assert.Equal(t, []string{"abcd", "abc", "ab", "a"}, strs)

	strs2 := []string{"中文", "测试中文", "字"}
	sortByLengthDesc(strs2)
	assert.Equal(t, []string{"测试中文", "中文", "字"}, strs2)
}

func BenchmarkRedactForLog(b *testing.B) {
	text := "User phone is 13812345678 and another phone is 13987654321"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RedactForLog(text, 100)
	}
}
