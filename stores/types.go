package stores

const (
	AccessLevelPublic = "PUBLIC"
	AccessLevelTrial  = "TRIAL"
	AccessLevelPaid   = "PAID"
)

const (
	PostStatusDraft     = "DRAFT"
	PostStatusScheduled = "SCHEDULED"
	PostStatusPublished = "PUBLISHED"
	PostStatusArchived  = "ARCHIVED"
)

const (
	CourseStatusDraft     = "DRAFT"
	CourseStatusScheduled = "SCHEDULED"
	CourseStatusPublished = "PUBLISHED"
	CourseStatusArchived  = "ARCHIVED"
)

const (
	ProblemDifficultyEasy   = "EASY"
	ProblemDifficultyMedium = "MEDIUM"
	ProblemDifficultyHard   = "HARD"
)

const (
	ProblemStatusDraft     = "DRAFT"
	ProblemStatusPublished = "PUBLISHED"
	ProblemStatusArchived  = "ARCHIVED"
)

const (
	DatasetTypePublic = "PUBLIC"
	DatasetTypeHidden = "HIDDEN"
)

const (
	ScoringModeBinary  = "BINARY"
	ScoringModePartial = "PARTIAL"
)

const (
	SubmissionModeRun    = "RUN"
	SubmissionModeSubmit = "SUBMIT"
)

const (
	SubmissionStatusQueued    = "QUEUED"
	SubmissionStatusRunning   = "RUNNING"
	SubmissionStatusCompleted = "COMPLETED"
	SubmissionStatusFailed    = "FAILED"
)

const (
	VerdictAccepted            = "AC"
	VerdictWrongAnswer         = "WRONG_ANSWER"
	VerdictTLE                 = "TLE"
	VerdictMLE                 = "MLE"
	VerdictRuntimeError        = "RUNTIME_ERROR"
	VerdictCompileError        = "COMPILE_ERROR"
	VerdictOutputLimitExceeded = "OUTPUT_LIMIT_EXCEEDED"
	VerdictInternalError       = "INTERNAL_ERROR"
	VerdictSkipped             = "SKIPPED"
)

const (
	ValidatorTypeExact          = "EXACT"
	ValidatorTypeJSONEquiv      = "JSON_EQUIV"
	ValidatorTypeUnorderedEquiv = "UNORDERED_EQUIV"
	ValidatorTypeFloatTolerance = "FLOAT_TOLERANCE"
	ValidatorTypeCustomChecker  = "CUSTOM_CHECKER"
)

const (
	TestcaseVisibilityPublic = "PUBLIC"
	TestcaseVisibilityHidden = "HIDDEN"
)

const (
	TestcaseGroupEdge   = "EDGE"
	TestcaseGroupNormal = "NORMAL"
	TestcaseGroupStress = "STRESS"
)

const (
	ExecutionOrderFastFirst   = "FAST_FIRST"
	ExecutionArtifactsMinimal = "MINIMAL"
	ExecutionArtifactsFull    = "FULL"
)

const (
	UserRoleUser       = "USER"
	UserRoleAdmin      = "ADMIN"
	UserRoleSuperAdmin = "SUPER_ADMIN"
)

const (
	UserStatusActive    = "ACTIVE"
	UserStatusSuspended = "SUSPENDED"
)

const (
	EntitlementStatusActive  = "ACTIVE"
	EntitlementStatusExpired = "EXPIRED"
)

const (
	PaymentStatusCreated = "CREATED"
	PaymentStatusPaid    = "PAID"
	PaymentStatusFailed  = "FAILED"
)

const (
	PromoStatusActive = "ACTIVE"
	PromoStatusPaused = "PAUSED"
)

const (
	PromoSlotInline          = "INLINE"
	PromoSlotBottomCard      = "BOTTOM_CARD"
	PromoSlotModalOnComplete = "MODAL_ON_COMPLETE"
	PromoSlotPaywallCard     = "PAYWALL_CARD"
)

const (
	FunnelStageNew         = "NEW"
	FunnelStageCasual      = "CASUAL"
	FunnelStageEngaged     = "ENGAGED"
	FunnelStageHot         = "HOT"
	FunnelStagePaidActive  = "PAID_ACTIVE"
	FunnelStagePaidExpired = "PAID_EXPIRED"
	FunnelStageDormant     = "DORMANT"
)
