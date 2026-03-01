package repository

import (
	"context"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// -- User Repository ------------------------------------------------

// UserRepository defines data access for users, schools, classes, and roles.
type UserRepository interface {
	// FindByPhone returns a user by phone number, preloading SchoolRoles.Role and SchoolRoles.School.
	FindByPhone(ctx context.Context, phone string) (*model.User, error)

	// FindByID returns a user by primary key, preloading SchoolRoles.Role and SchoolRoles.School.
	FindByID(ctx context.Context, id uint) (*model.User, error)

	// FindByIDs returns users matching the given IDs with selected fields (e.g., "id, display_name").
	FindByIDs(ctx context.Context, ids []uint, selectFields string) ([]model.User, error)

	// ListUsers returns paginated users, optionally filtered by school.
	// When schoolID > 0, only users belonging to that school are returned.
	ListUsers(ctx context.Context, schoolID uint, offset, limit int) ([]model.User, int64, error)

	// CreateUserWithRole creates a user and its role assignment in a transaction.
	CreateUserWithRole(ctx context.Context, user *model.User, role *model.UserSchoolRole) error

	// FindRoleByName returns a role by its name.
	FindRoleByName(ctx context.Context, name model.RoleName) (*model.Role, error)

	// FindStudentClassIDs returns the class IDs a student belongs to.
	FindStudentClassIDs(ctx context.Context, studentID uint) ([]uint, error)

	// FindStudentIDsByClassID returns student IDs for a given class.
	FindStudentIDsByClassID(ctx context.Context, classID uint) ([]uint, error)

	// ListSchools returns paginated schools.
	ListSchools(ctx context.Context, offset, limit int) ([]model.School, int64, error)

	// CreateSchool creates a new school.
	CreateSchool(ctx context.Context, school *model.School) error

	// ListClasses returns paginated classes, optionally filtered by school.
	ListClasses(ctx context.Context, schoolID uint, offset, limit int) ([]model.Class, int64, error)

	// CreateClass creates a new class.
	CreateClass(ctx context.Context, class *model.Class) error
}

// -- Course Repository ----------------------------------------------

// CourseRepository defines data access for courses.
type CourseRepository interface {
	// FindByID returns a course by primary key.
	FindByID(ctx context.Context, id uint) (*model.Course, error)

	// FindWithOutline returns a course with chapters (sorted by sort_order),
	// knowledge points, and mounted skills preloaded.
	FindWithOutline(ctx context.Context, id uint) (*model.Course, error)

	// FindWithChaptersAndKPs returns a course with chapters and knowledge points preloaded.
	FindWithChaptersAndKPs(ctx context.Context, id uint) (*model.Course, error)

	// ListByTeacher returns paginated courses for a teacher, optionally filtered by school.
	// Preloads Chapters.KnowledgePoints.
	ListByTeacher(ctx context.Context, teacherID uint, schoolID uint, offset, limit int) ([]model.Course, int64, error)

	// Create creates a new course.
	Create(ctx context.Context, course *model.Course) error
}

// -- Document Repository --------------------------------------------

// DocumentRepository defines data access for documents and chunks.
type DocumentRepository interface {
	// Create creates a new document record.
	Create(ctx context.Context, doc *model.Document) error

	// UpdateStatus updates the status of a document.
	UpdateStatus(ctx context.Context, docID uint, status model.DocStatus) error

	// UpdateFields updates arbitrary fields on a document.
	UpdateFields(ctx context.Context, docID uint, fields map[string]interface{}) error

	// FindByCourseID returns all documents for a course.
	FindByCourseID(ctx context.Context, courseID uint) ([]model.Document, error)

	// FindByCourseIDOrdered returns all documents for a course, ordered by created_at DESC.
	FindByCourseIDOrdered(ctx context.Context, courseID uint) ([]model.Document, error)

	// FindByIDAndCourseID returns a document matching both ID and course ID.
	FindByIDAndCourseID(ctx context.Context, docID, courseID uint) (*model.Document, error)

	// DeleteChunksByDocumentID deletes all chunks belonging to a document.
	DeleteChunksByDocumentID(ctx context.Context, docID uint) error

	// Delete deletes a document record.
	Delete(ctx context.Context, doc *model.Document) error
}

// -- Activity Repository --------------------------------------------

// ActivityRepository defines data access for learning activities.
type ActivityRepository interface {
	// FindByID returns a learning activity by primary key.
	FindByID(ctx context.Context, id uint) (*model.LearningActivity, error)

	// Create creates a new learning activity.
	Create(ctx context.Context, activity *model.LearningActivity) error

	// CreateClassAssignment creates a class-activity assignment.
	CreateClassAssignment(ctx context.Context, assignment *model.ActivityClassAssignment) error

	// ListByTeacher returns paginated activities for a teacher, with optional filters.
	ListByTeacher(ctx context.Context, teacherID uint, courseID uint, status string, offset, limit int) ([]model.LearningActivity, int64, error)

	// UpdateFields updates arbitrary fields on an activity.
	UpdateFields(ctx context.Context, activityID uint, fields map[string]interface{}) error

	// ListPublishedForClasses returns published activities assigned to the given class IDs.
	ListPublishedForClasses(ctx context.Context, classIDs []uint) ([]model.LearningActivity, error)
}

// -- Session Repository ---------------------------------------------

// SessionRepository defines data access for student sessions and interactions.
type SessionRepository interface {
	// FindByID returns a session by primary key.
	FindByID(ctx context.Context, id uint) (*model.StudentSession, error)

	// FindActive returns an active session matching the criteria.
	FindActive(ctx context.Context, studentID, activityID uint, isSandbox bool) (*model.StudentSession, error)

	// Create creates a new student session.
	Create(ctx context.Context, session *model.StudentSession) error

	// ListByActivityID returns all sessions for an activity, optionally excluding sandbox sessions.
	ListByActivityID(ctx context.Context, activityID uint, excludeSandbox bool) ([]model.StudentSession, error)

	// FindInteractions returns interactions for a session, ordered by created_at ASC.
	FindInteractions(ctx context.Context, sessionID uint, limit int) ([]model.Interaction, error)

	// CountStudentInteractions counts student-role interactions in a session.
	CountStudentInteractions(ctx context.Context, sessionID uint) (int64, error)
}

// -- Mastery Repository ---------------------------------------------

// MasteryRepository defines data access for student mastery and error notebook.
type MasteryRepository interface {
	// FindByStudent returns mastery records for a student, optionally filtered by KP IDs.
	FindByStudent(ctx context.Context, studentID uint, kpIDs []uint) ([]model.StudentKPMastery, error)

	// FindByStudentsAndKPs returns mastery records for given students and knowledge points.
	FindByStudentsAndKPs(ctx context.Context, studentIDs, kpIDs []uint) ([]model.StudentKPMastery, error)

	// FindByKPIDs returns all mastery records for the given knowledge points.
	FindByKPIDs(ctx context.Context, kpIDs []uint) ([]model.StudentKPMastery, error)

	// AggregateAvgByKP returns the average mastery score and count for a KP,
	// optionally filtered to specific students.
	AggregateAvgByKP(ctx context.Context, kpID uint, studentIDs []uint) (avg float64, count int64, err error)

	// CountDistinctStudents counts the distinct students with mastery records for the given KPs.
	CountDistinctStudents(ctx context.Context, kpIDs []uint, studentIDs []uint) (int64, error)

	// AggregateDailyMastery returns daily aggregated mastery for a student.
	AggregateDailyMastery(ctx context.Context, studentID uint, kpIDs []uint) ([]DailyMasteryAgg, error)

	// FindMastered returns mastery records where mastery_score >= threshold.
	FindMastered(ctx context.Context, studentID uint, threshold float64) ([]model.StudentKPMastery, error)

	// ListErrorNotebook returns error notebook entries for a student with optional filters.
	ListErrorNotebook(ctx context.Context, studentID uint, resolved *bool, kpID uint) ([]model.ErrorNotebookEntry, error)

	// CountErrorNotebook returns total and unresolved error notebook counts for a student.
	CountErrorNotebook(ctx context.Context, studentID uint) (total, unresolved int64, err error)

	// FindErrorNotebookByKPIDs returns error notebook entries for the given KP IDs.
	FindErrorNotebookByKPIDs(ctx context.Context, kpIDs []uint) ([]model.ErrorNotebookEntry, error)
}

// DailyMasteryAgg represents a daily aggregated mastery data point.
type DailyMasteryAgg struct {
	Date     string  `json:"date"`
	AvgScore float64 `json:"avg_score"`
	Count    int     `json:"count"`
}

// -- Knowledge Point Repository -------------------------------------

// KnowledgePointRepository defines data access for knowledge points, misconceptions, and cross-links.
type KnowledgePointRepository interface {
	// FindByID returns a knowledge point by primary key.
	FindByID(ctx context.Context, id uint) (*model.KnowledgePoint, error)

	// FindByCourseID returns all KPs for a course (via JOIN on chapters), ordered.
	FindByCourseID(ctx context.Context, courseID uint) ([]model.KnowledgePoint, error)

	// FindIDsByCourseID returns just the KP IDs for a course.
	FindIDsByCourseID(ctx context.Context, courseID uint) ([]uint, error)

	// FindByIDs returns knowledge points matching the given IDs.
	FindByIDs(ctx context.Context, ids []uint) ([]model.KnowledgePoint, error)

	// FindByIDsWithChapter returns KPs with their Chapter preloaded.
	FindByIDsWithChapter(ctx context.Context, ids []uint) ([]model.KnowledgePoint, error)

	// FindWithChapterTitles returns KP ID, title, and chapter title for the given IDs.
	FindWithChapterTitles(ctx context.Context, ids []uint) ([]KPWithChapterTitle, error)

	// CreateMisconception creates a new misconception.
	CreateMisconception(ctx context.Context, m *model.Misconception) error

	// UpdateMisconceptionNeo4jID updates the Neo4j node ID on a misconception.
	UpdateMisconceptionNeo4jID(ctx context.Context, id uint, nodeID string) error

	// ListMisconceptionsByKPID returns misconceptions for a KP, ordered by severity DESC.
	ListMisconceptionsByKPID(ctx context.Context, kpID uint) ([]model.Misconception, error)

	// FindMisconceptionByIDAndKPID returns a misconception matching both ID and KP ID.
	FindMisconceptionByIDAndKPID(ctx context.Context, id, kpID uint) (*model.Misconception, error)

	// DeleteMisconception deletes a misconception.
	DeleteMisconception(ctx context.Context, m *model.Misconception) error

	// CreateCrossLink creates a new cross-link.
	CreateCrossLink(ctx context.Context, link *model.CrossLink) error

	// ListCrossLinksByKPID returns cross-links where the KP appears on either side.
	ListCrossLinksByKPID(ctx context.Context, kpID uint) ([]model.CrossLink, error)

	// FindCrossLinkByIDAndKPID returns a cross-link by ID where the KP appears on either side.
	FindCrossLinkByIDAndKPID(ctx context.Context, linkID, kpID uint) (*model.CrossLink, error)

	// DeleteCrossLink deletes a cross-link.
	DeleteCrossLink(ctx context.Context, link *model.CrossLink) error
}

// KPWithChapterTitle is a projection of KnowledgePoint with its chapter title.
type KPWithChapterTitle struct {
	ID           uint   `gorm:"column:id"`
	Title        string `gorm:"column:title"`
	ChapterTitle string `gorm:"column:chapter_title"`
}

// -- Achievement Repository -----------------------------------------

// AchievementRepository defines data access for achievements.
type AchievementRepository interface {
	// ListDefinitions returns all achievement definitions ordered by sort_order.
	ListDefinitions(ctx context.Context) ([]model.AchievementDefinition, error)

	// ListDefinitionsByType returns definitions of a given type ordered by threshold.
	ListDefinitionsByType(ctx context.Context, achievementType model.AchievementType) ([]model.AchievementDefinition, error)

	// FindStudentAchievements returns all achievement records for a student.
	FindStudentAchievements(ctx context.Context, studentID uint) ([]model.StudentAchievement, error)

	// FindStudentAchievement returns a specific student-achievement record.
	FindStudentAchievement(ctx context.Context, studentID, achievementID uint) (*model.StudentAchievement, error)

	// CreateStudentAchievement creates a new student achievement record.
	CreateStudentAchievement(ctx context.Context, rec *model.StudentAchievement) error

	// SaveStudentAchievement performs a full update (save) on an achievement record.
	SaveStudentAchievement(ctx context.Context, rec *model.StudentAchievement) error
}

// -- Marketplace Repository -----------------------------------------

// MarketplaceRepository defines data access for marketplace plugins.
type MarketplaceRepository interface {
	// ListApproved returns paginated approved plugins with optional filters.
	ListApproved(ctx context.Context, pluginType, category, search string, offset, limit int) ([]model.MarketplacePlugin, int64, error)

	// FindByPluginID returns a plugin by its plugin_id field.
	FindByPluginID(ctx context.Context, pluginID string) (*model.MarketplacePlugin, error)

	// FindApprovedByPluginID returns an approved plugin by its plugin_id.
	FindApprovedByPluginID(ctx context.Context, pluginID string) (*model.MarketplacePlugin, error)

	// ListReviewsByPluginID returns reviews for a plugin, limited to `limit` entries.
	ListReviewsByPluginID(ctx context.Context, pluginID string, limit int) ([]model.MarketplaceReview, error)

	// CreatePlugin creates a new marketplace plugin.
	CreatePlugin(ctx context.Context, plugin *model.MarketplacePlugin) error

	// FindInstalledPlugin returns an installed plugin by school and plugin ID.
	FindInstalledPlugin(ctx context.Context, schoolID uint, pluginID string) (*model.InstalledPlugin, error)

	// CreateInstalledPlugin creates a new installed plugin record.
	CreateInstalledPlugin(ctx context.Context, installed *model.InstalledPlugin) error

	// IncrementDownloads atomically increments the download count for a plugin.
	IncrementDownloads(ctx context.Context, pluginID uint) error

	// DeleteInstalledPlugin deletes an installed plugin by its ID.
	DeleteInstalledPlugin(ctx context.Context, id uint) error

	// ListInstalledBySchool returns all installed plugins for a school.
	ListInstalledBySchool(ctx context.Context, schoolID uint) ([]model.InstalledPlugin, error)
}
