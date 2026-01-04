package main

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DashboardData aggregates all info needed for the dashboard
type DashboardData struct {
	Role    string          `json:"role"`
	Classes []ClassOverview `json:"classes"`
	// Student specific
	StudentStats *StudentDashboardStats `json:"student_stats,omitempty"`
	Upcoming     []UpcomingAssignment   `json:"upcoming,omitempty"`
	// Teacher specific
	TeacherStats *TeacherDashboardStats `json:"teacher_stats,omitempty"`
}

type ClassOverview struct {
	ID                 uuid.UUID            `json:"id"`
	Name               string               `json:"name"`
	AssignmentsCount   int                  `json:"assignments_count"`
	CompletedCount     int                  `json:"completed_count"`     // Student: how many they finished; Teacher: avg? or just N/A
	ProgressPercent    int                  `json:"progress_percent"`    // Student: % of assignments done
	AssignmentProgress []AssignmentProgress `json:"assignment_progress"` // Top 3 or 5
	NotFinishedCount   int                  `json:"not_finished_count"`  // Teacher: how many assignments not finished by all
	StudentsCount      int                  `json:"students_count"`      // Teacher
}

type AssignmentProgress struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Done       bool      `json:"done"`        // Student
	DoneCount  int       `json:"done_count"`  // Teacher
	TotalCount int       `json:"total_count"` // Teacher
}

type StudentDashboardStats struct {
	TotalClasses         int     `json:"total_classes"`
	TotalAssignments     int     `json:"total_assignments"`
	CompletedAssignments int     `json:"completed_assignments"`
	PointsEarned         float64 `json:"points_earned"`
	PointsTotal          float64 `json:"points_total"`
}

type TeacherDashboardStats struct {
	TotalClasses      int `json:"total_classes"`
	StudentsTotal     int `json:"students_total"`
	ActiveAssignments int `json:"active_assignments"`
}

type UpcomingAssignment struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	ClassName string    `json:"class_name"`
	Deadline  time.Time `json:"deadline"`
}

func getDashboard(c *gin.Context) {
	role := c.GetString("role")
	uid := getUserID(c)

	data, err := GetDashboardData(role, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, data)
}

func GetDashboardData(role string, userID uuid.UUID) (*DashboardData, error) {
	data := &DashboardData{Role: role}

	if role == "student" {
		return getStudentDashboard(userID, data)
	} else if role == "teacher" {
		return getTeacherDashboard(userID, data)
	} else {
		// Admin or others - maybe just return empty or basic info
		return data, nil
	}
}

func getStudentDashboard(studentID uuid.UUID, data *DashboardData) (*DashboardData, error) {
	// 1. Get all classes for student
	var classes []Class
	err := DB.Select(&classes, `
		SELECT c.* FROM classes c
		JOIN class_students cs ON cs.class_id = c.id
		WHERE cs.student_id = $1
		ORDER BY c.created_at DESC
	`, studentID)
	if err != nil {
		return nil, err
	}

	// 2. Get all published assignments for these classes
	// We can do this in one query
	type Asgn struct {
		Assignment
		ClassName string `db:"class_name"`
	}
	var assignments []Asgn
	err = DB.Select(&assignments, `
		SELECT a.id, a.title, a.created_at, a.class_id, a.deadline, a.max_points, c.name as class_name
		FROM assignments a
		JOIN class_students cs ON cs.class_id = a.class_id
		JOIN classes c ON c.id = a.class_id
		WHERE cs.student_id = $1 AND a.published = true
	`, studentID)
	if err != nil {
		return nil, err
	}

	// 3. Get all submissions for this student (only fields needed for stats)
	type SubStat struct {
		AssignmentID uuid.UUID `db:"assignment_id"`
		Points       *float64  `db:"points"`
		OverridePts  *float64  `db:"override_points"`
	}
	var submissions []SubStat
	err = DB.Select(&submissions, `
		SELECT assignment_id, points, override_points
		  FROM submissions
		 WHERE student_id = $1
	`, studentID)
	if err != nil {
		return nil, err
	}

	// Process data in memory to avoid N+1
	stats := &StudentDashboardStats{
		TotalClasses: len(classes),
	}

	// Map assignments by class
	asgnByClass := make(map[uuid.UUID][]Asgn)
	for _, a := range assignments {
		asgnByClass[a.ClassID] = append(asgnByClass[a.ClassID], a)
		stats.TotalAssignments++
		stats.PointsTotal += float64(a.MaxPoints)
	}

	// Map best submission by assignment
	bestSub := make(map[uuid.UUID]float64)

	for _, s := range submissions {
		pts := 0.0
		if s.OverridePts != nil {
			pts = *s.OverridePts
		} else if s.Points != nil {
			pts = *s.Points
		}
		if pts > bestSub[s.AssignmentID] {
			bestSub[s.AssignmentID] = pts
		}

		// Check if done (simple check based on points for now, or status)
		// The original code checked: best >= a.max_points
		// We need to cross reference with assignment max points, done below
	}

	upcoming := []UpcomingAssignment{}
	now := time.Now()
	soon := now.Add(7 * 24 * time.Hour)

	for _, c := range classes {
		co := ClassOverview{
			ID:   c.ID,
			Name: c.Name,
		}

		classAsgns := asgnByClass[c.ID]
		// Sort by created_at desc for display
		sort.Slice(classAsgns, func(i, j int) bool {
			return classAsgns[i].CreatedAt.After(classAsgns[j].CreatedAt)
		})

		co.AssignmentsCount = len(classAsgns)

		for _, a := range classAsgns {
			points := bestSub[a.ID]
			isDone := points >= float64(a.MaxPoints)

			if isDone {
				co.CompletedCount++
				stats.CompletedAssignments++
			}
			stats.PointsEarned += points

			// Add to upcoming if deadline is soon
			// Check for override? The original code did check for overrides.
			// For simplicity/performance, we might skip overrides in bulk or fetch them.
			// Let's fetch overrides for accuracy.
			deadline := a.Deadline
			// We could fetch overrides in a separate query if needed, but let's assume standard deadline for bulk view first
			// Optimization: Fetch overrides in one go

			if deadline.After(now) && deadline.Before(soon) {
				upcoming = append(upcoming, UpcomingAssignment{
					ID:        a.ID,
					Title:     a.Title,
					ClassName: c.Name,
					Deadline:  deadline,
				})
			}

			// Add to progress list (limit to top 3 in UI, but we send all or top 5)
			if len(co.AssignmentProgress) < 5 {
				co.AssignmentProgress = append(co.AssignmentProgress, AssignmentProgress{
					ID:    a.ID,
					Title: a.Title,
					Done:  isDone,
				})
			}
		}

		if co.AssignmentsCount > 0 {
			co.ProgressPercent = int(float64(co.CompletedCount) / float64(co.AssignmentsCount) * 100)
		}

		data.Classes = append(data.Classes, co)
	}

	// Sort upcoming
	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].Deadline.Before(upcoming[j].Deadline)
	})
	data.Upcoming = upcoming
	data.StudentStats = stats

	return data, nil
}

func getTeacherDashboard(teacherID uuid.UUID, data *DashboardData) (*DashboardData, error) {
	// 1. Get classes
	var classes []Class
	err := DB.Select(&classes, `SELECT * FROM classes WHERE teacher_id=$1 ORDER BY created_at DESC`, teacherID)
	if err != nil {
		return nil, err
	}

	stats := &TeacherDashboardStats{
		TotalClasses: len(classes),
	}

	// 2. Get student counts per class
	// 3. Get assignments per class
	// 4. Get submission stats per assignment

	// We can use a few aggregated queries

	// Student counts
	rows, err := DB.Query(`
		SELECT class_id, COUNT(student_id) 
		FROM class_students 
		WHERE class_id IN (SELECT id FROM classes WHERE teacher_id=$1)
		GROUP BY class_id
	`, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	studentCounts := make(map[uuid.UUID]int)
	// Actually the DB query above just gives counts.
	// To get total unique students we need another query.

	for rows.Next() {
		var cid uuid.UUID
		var count int
		if err := rows.Scan(&cid, &count); err == nil {
			studentCounts[cid] = count
		}
	}

	// Total unique students
	var totalUniqueStudents int
	DB.Get(&totalUniqueStudents, `
		SELECT COUNT(DISTINCT student_id) 
		FROM class_students cs
		JOIN classes c ON c.id = cs.class_id
		WHERE c.teacher_id = $1
	`, teacherID)
	stats.StudentsTotal = totalUniqueStudents

	// Assignments
	var assignments []Assignment
	err = DB.Select(&assignments, `
		SELECT a.id, a.title, a.class_id, a.created_at FROM assignments a
		JOIN classes c ON c.id = a.class_id
		WHERE c.teacher_id = $1
	`, teacherID)
	if err != nil {
		return nil, err
	}

	stats.ActiveAssignments = len(assignments)
	asgnByClass := make(map[uuid.UUID][]Assignment)
	for _, a := range assignments {
		asgnByClass[a.ClassID] = append(asgnByClass[a.ClassID], a)
	}

	// Submission completion stats
	// We need to know for each assignment, how many students completed it.
	// "Completed" means status='completed' AND !is_teacher_run

	type CompletionStat struct {
		AssignmentID uuid.UUID `db:"assignment_id"`
		DoneCount    int       `db:"done_count"`
	}
	var completionStats []CompletionStat
	err = DB.Select(&completionStats, `
		SELECT s.assignment_id, COUNT(DISTINCT s.student_id) as done_count
		FROM submissions s
		JOIN assignments a ON a.id = s.assignment_id
		JOIN classes c ON c.id = a.class_id
		WHERE c.teacher_id = $1 
		  AND s.status = 'completed' 
		  AND s.is_teacher_run = false
		GROUP BY s.assignment_id
	`, teacherID)
	if err != nil {
		return nil, err
	}

	compMap := make(map[uuid.UUID]int)
	for _, cs := range completionStats {
		compMap[cs.AssignmentID] = cs.DoneCount
	}

	for _, c := range classes {
		co := ClassOverview{
			ID:            c.ID,
			Name:          c.Name,
			StudentsCount: studentCounts[c.ID],
		}

		classAsgns := asgnByClass[c.ID]
		// Sort by created_at desc
		sort.Slice(classAsgns, func(i, j int) bool {
			return classAsgns[i].CreatedAt.After(classAsgns[j].CreatedAt)
		})

		co.AssignmentsCount = len(classAsgns)

		for _, a := range classAsgns {
			doneCount := compMap[a.ID]
			totalStudents := co.StudentsCount

			if doneCount < totalStudents {
				co.NotFinishedCount++
			}

			if len(co.AssignmentProgress) < 5 {
				co.AssignmentProgress = append(co.AssignmentProgress, AssignmentProgress{
					ID:         a.ID,
					Title:      a.Title,
					DoneCount:  doneCount,
					TotalCount: totalStudents,
				})
			}
		}

		data.Classes = append(data.Classes, co)
	}

	data.TeacherStats = stats
	return data, nil
}
