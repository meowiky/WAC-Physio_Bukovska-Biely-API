package physio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

var errNotFound = errors.New("document not found")
var errConflict = errors.New("document already exists")

type Server struct {
	client              *mongo.Client
	timeout             time.Duration
	patients            *mongo.Collection
	therapists          *mongo.Collection
	ambulances          *mongo.Collection
	rehabilitationPlans *mongo.Collection
	rehabSessions       *mongo.Collection
}

func NewServer(ctx context.Context) (*Server, error) {
	host := envOrDefault("PHYSIO_API_MONGODB_HOST", "localhost")
	port := envIntOrDefault("PHYSIO_API_MONGODB_PORT", 27017)
	username := envOrDefault("PHYSIO_API_MONGODB_USERNAME", "")
	password := envOrDefault("PHYSIO_API_MONGODB_PASSWORD", "")
	dbName := envOrDefault("PHYSIO_API_MONGODB_DATABASE", "wac-physio")
	timeout := envDurationSecondsOrDefault("PHYSIO_API_MONGODB_TIMEOUT_SECONDS", 10*time.Second)

	uri := fmt.Sprintf("mongodb://%s:%d", host, port)
	if username != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d", username, password, host, port)
	}

	client, err := mongo.Connect(
		options.Client().
			ApplyURI(uri).
			SetConnectTimeout(timeout).
			SetServerSelectionTimeout(timeout),
	)
	if err != nil {
		return nil, err
	}

	connectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := client.Ping(connectCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}

	db := client.Database(dbName)
	return &Server{
		client:              client,
		timeout:             timeout,
		patients:            db.Collection("patients"),
		therapists:          db.Collection("therapists"),
		ambulances:          db.Collection("ambulances"),
		rehabilitationPlans: db.Collection("rehabilitation_plans"),
		rehabSessions:       db.Collection("rehabilitation_sessions"),
	}, nil
}

func (s *Server) Disconnect(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Disconnect(ctx)
}

func (s *Server) requestContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, s.timeout)
}

func envOrDefault(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	value := envOrDefault(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDurationSecondsOrDefault(key string, fallback time.Duration) time.Duration {
	value := envOrDefault(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return time.Duration(parsed) * time.Second
}

func newDocumentID() string {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err == nil {
		return hex.EncodeToString(random)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 16)
}

func jsonError(c *gin.Context, status int, message string, err error) {
	payload := gin.H{
		"status":  status,
		"message": message,
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	c.JSON(status, payload)
}

func createDocument[T any](ctx context.Context, collection *mongo.Collection, id string, document *T) error {
	result := collection.FindOne(ctx, bson.M{"id": id})
	switch result.Err() {
	case nil:
		return errConflict
	case mongo.ErrNoDocuments:
	default:
		return result.Err()
	}

	_, err := collection.InsertOne(ctx, document)
	return err
}

func findDocumentByID[T any](ctx context.Context, collection *mongo.Collection, id string) (*T, error) {
	var document T
	result := collection.FindOne(ctx, bson.M{"id": id})
	switch result.Err() {
	case nil:
	case mongo.ErrNoDocuments:
		return nil, errNotFound
	default:
		return nil, result.Err()
	}

	if err := result.Decode(&document); err != nil {
		return nil, err
	}
	return &document, nil
}

func replaceDocument[T any](ctx context.Context, collection *mongo.Collection, id string, document *T) error {
	result, err := collection.ReplaceOne(ctx, bson.M{"id": id}, document)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return errNotFound
	}
	return nil
}

func deleteDocument(ctx context.Context, collection *mongo.Collection, id string) error {
	result, err := collection.DeleteOne(ctx, bson.M{"id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return errNotFound
	}
	return nil
}

func listDocuments[T any](ctx context.Context, collection *mongo.Collection) ([]T, error) {
	cursor, err := collection.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var documents []T
	if err := cursor.All(ctx, &documents); err != nil {
		return nil, err
	}
	if documents == nil {
		return []T{}, nil
	}
	return documents, nil
}

func parseDateTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}

func overlaps(startA, endA, startB, endB time.Time) bool {
	return startA.Before(endB) && startB.Before(endA)
}

func (s *Server) sessions(ctx context.Context) ([]RehabilitationSession, error) {
	return listDocuments[RehabilitationSession](ctx, s.rehabSessions)
}

func (s *Server) plans(ctx context.Context) ([]RehabilitationPlan, error) {
	return listDocuments[RehabilitationPlan](ctx, s.rehabilitationPlans)
}

func (s *Server) patientsList(ctx context.Context) ([]Patient, error) {
	return listDocuments[Patient](ctx, s.patients)
}

func (s *Server) therapistsList(ctx context.Context) ([]Therapist, error) {
	return listDocuments[Therapist](ctx, s.therapists)
}

func (s *Server) ambulancesList(ctx context.Context) ([]Ambulance, error) {
	return listDocuments[Ambulance](ctx, s.ambulances)
}

func validatePatient(patient Patient) error {
	switch {
	case patient.FirstName == "":
		return fmt.Errorf("firstName is required")
	case patient.Surname == "":
		return fmt.Errorf("surname is required")
	case patient.BirthNumber == "":
		return fmt.Errorf("birthNumber is required")
	case patient.DateOfBirth == "":
		return fmt.Errorf("dateOfBirth is required")
	case patient.Email == "":
		return fmt.Errorf("email is required")
	case patient.Phone == "":
		return fmt.Errorf("phone is required")
	case patient.FirstVisitDate == "":
		return fmt.Errorf("firstVisitDate is required")
	}

	if patient.Sex != MALE && patient.Sex != FEMALE {
		return fmt.Errorf("sex must be one of: %s, %s", MALE, FEMALE)
	}
	if patient.HealthInsurance != UNION && patient.HealthInsurance != VSZP && patient.HealthInsurance != DOVERA {
		return fmt.Errorf("healthInsurance must be one of: %s, %s, %s", UNION, VSZP, DOVERA)
	}
	return nil
}

func validateTherapist(therapist Therapist) error {
	switch {
	case therapist.FirstName == "":
		return fmt.Errorf("firstName is required")
	case therapist.Surname == "":
		return fmt.Errorf("surname is required")
	case therapist.Email == "":
		return fmt.Errorf("email is required")
	case therapist.Phone == "":
		return fmt.Errorf("phone is required")
	}
	return nil
}

func validateAmbulance(ambulance Ambulance) error {
	switch {
	case ambulance.Name == "":
		return fmt.Errorf("name is required")
	case ambulance.RoomNumber == "":
		return fmt.Errorf("roomNumber is required")
	}
	return nil
}

func validatePlan(plan RehabilitationPlan) error {
	switch {
	case plan.PatientId == "":
		return fmt.Errorf("patientId is required")
	case plan.Status != DRAFT && plan.Status != ACTIVE && plan.Status != COMPLETED && plan.Status != PLAN_CANCELED:
		return fmt.Errorf("status must be one of: %s, %s, %s, %s", DRAFT, ACTIVE, COMPLETED, PLAN_CANCELED)
	}
	return nil
}

func validateSession(session RehabilitationSession) error {
	switch {
	case session.PlanId == "":
		return fmt.Errorf("planId is required")
	case session.AmbulanceId == "":
		return fmt.Errorf("ambulanceId is required")
	case session.TherapistId == "":
		return fmt.Errorf("therapistId is required")
	}

	if session.AttendanceStatus != PLANNED && session.AttendanceStatus != ATTENDED && session.AttendanceStatus != CANCELED {
		return fmt.Errorf("attendanceStatus must be one of: %s, %s, %s", PLANNED, ATTENDED, CANCELED)
	}
	if session.ConfirmationStatus != TENTATIVE && session.ConfirmationStatus != CONFIRMED {
		return fmt.Errorf("confirmationStatus must be one of: %s, %s", TENTATIVE, CONFIRMED)
	}

	if (session.StartDateTime == nil) != (session.EndDateTime == nil) {
		return fmt.Errorf("startDateTime and endDateTime must either both be set or both be empty")
	}
	if session.StartDateTime != nil && !session.StartDateTime.Before(*session.EndDateTime) {
		return fmt.Errorf("startDateTime must be before endDateTime")
	}
	return nil
}

func (s *Server) patientExists(ctx context.Context, patientID string) (bool, error) {
	_, err := findDocumentByID[Patient](ctx, s.patients, patientID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return false, err
}

func (s *Server) therapistExists(ctx context.Context, therapistID string) (bool, error) {
	_, err := findDocumentByID[Therapist](ctx, s.therapists, therapistID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return false, err
}

func (s *Server) ambulanceExists(ctx context.Context, ambulanceID string) (bool, error) {
	_, err := findDocumentByID[Ambulance](ctx, s.ambulances, ambulanceID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return false, err
}

func (s *Server) planExists(ctx context.Context, planID string) (bool, error) {
	_, err := findDocumentByID[RehabilitationPlan](ctx, s.rehabilitationPlans, planID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return false, err
}

func (s *Server) patientHasPlans(ctx context.Context, patientID string) (bool, error) {
	plans, err := s.plans(ctx)
	if err != nil {
		return false, err
	}
	for _, plan := range plans {
		if plan.PatientId == patientID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) planHasSessions(ctx context.Context, planID string) (bool, error) {
	sessions, err := s.sessions(ctx)
	if err != nil {
		return false, err
	}
	for _, session := range sessions {
		if session.PlanId == planID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) therapistHasSessions(ctx context.Context, therapistID string) (bool, error) {
	sessions, err := s.sessions(ctx)
	if err != nil {
		return false, err
	}
	for _, session := range sessions {
		if session.TherapistId == therapistID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) ambulanceHasSessions(ctx context.Context, ambulanceID string) (bool, error) {
	sessions, err := s.sessions(ctx)
	if err != nil {
		return false, err
	}
	for _, session := range sessions {
		if session.AmbulanceId == ambulanceID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) patientBirthNumberConflict(ctx context.Context, patient Patient, excludeID string) (bool, error) {
	patients, err := s.patientsList(ctx)
	if err != nil {
		return false, err
	}
	for _, existing := range patients {
		if existing.Id != excludeID && existing.BirthNumber == patient.BirthNumber {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) availabilityForInterval(
	ctx context.Context,
	startDateTime time.Time,
	endDateTime time.Time,
	ambulanceID string,
	therapistID string,
	excludeSessionID string,
) (*RehabilitationSession, *RehabilitationSession, error) {
	sessions, err := s.sessions(ctx)
	if err != nil {
		return nil, nil, err
	}

	var ambulanceConflict *RehabilitationSession
	var therapistConflict *RehabilitationSession
	for _, session := range sessions {
		if session.Id == excludeSessionID {
			continue
		}
		if session.StartDateTime == nil || session.EndDateTime == nil {
			continue
		}
		if !overlaps(startDateTime, endDateTime, *session.StartDateTime, *session.EndDateTime) {
			continue
		}
		if ambulanceConflict == nil && ambulanceID != "" && session.AmbulanceId == ambulanceID {
			conflict := session
			ambulanceConflict = &conflict
		}
		if therapistConflict == nil && therapistID != "" && session.TherapistId == therapistID {
			conflict := session
			therapistConflict = &conflict
		}
		if ambulanceConflict != nil && therapistConflict != nil {
			break
		}
	}
	return ambulanceConflict, therapistConflict, nil
}

func idMismatch(c *gin.Context, pathID string, bodyID string) bool {
	if bodyID == "" {
		jsonError(c, http.StatusBadRequest, "id is required", nil)
		return true
	}
	if bodyID != pathID {
		jsonError(c, http.StatusForbidden, "path id does not match body id", nil)
		return true
	}
	return false
}
