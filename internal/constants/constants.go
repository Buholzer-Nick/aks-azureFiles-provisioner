package constants

const (
	// Annotation Keys
	ShareOverrideAnnotation = "kliggo.ch/share-override"
	ShareNameAnnotation     = "kliggo.ch/share-name"
	RetainShareAnnotation   = "kliggo.ch/retain-share"

	// Finalizers
	FinalizerName = "kliggo.ch/azurefile-provisioner"

	// Event Reasons
	EventShareEnsuring      = "ShareEnsuring"
	EventShareReady         = "ShareReady"
	EventShareError         = "ShareError"
	EventShareValidation    = "ShareValidationError"
	EventShareNameInvalid   = "ShareNameInvalid"
	EventPVCInvalid         = "PVCInvalid"
	EventShareClientMissing = "ShareClientMissing"
	EventPVBuildError       = "PVBuildError"
	EventCleanupStarted     = "CleanupStarted"
	EventShareDeleted       = "ShareDeleted"
	EventShareRetained      = "ShareRetained"
	EventCleanupComplete    = "CleanupComplete"
	EventPVCreated          = "PVCreated"
	EventPVMismatch         = "PVMismatch"
	EventPVAlreadyExists    = "PVAlreadyExists"

	// Drivers
	AzureFileCSIDriver = "file.csi.azure.com"
)
