package grappadcim

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/authz"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const component = "grappa-dcim"

type Deps struct {
	Grappa       *sql.DB
	Logger       *slog.Logger
	AppVersion   string
	ArtifactRoot string
}

type Handler struct {
	grappa       *sql.DB
	logger       *slog.Logger
	appVersion   string
	artifactRoot string
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	h := &Handler{
		grappa:       deps.Grappa,
		logger:       logger.With("component", component),
		appVersion:   deps.AppVersion,
		artifactRoot: cleanArtifactRoot(deps.ArtifactRoot),
	}

	readProtect := acl.RequireRole(applaunch.GrappaDCIMAccessRoles()...)
	handleRead := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, readProtect(http.HandlerFunc(handler)))
	}

	handleRead("GET /grappa-dcim/v1/meta", h.handleMeta)
	handleRead("GET /grappa-dcim/v1/lookups", h.handleLookups)
	handleRead("GET /grappa-dcim/v1/facilities/buildings", h.handleListBuildings)
	handleRead("GET /grappa-dcim/v1/facilities/buildings/{id}", h.handleGetBuilding)
	handleRead("GET /grappa-dcim/v1/facilities/datacenters", h.handleListDatacenters)
	handleRead("GET /grappa-dcim/v1/facilities/datacenters/{id}", h.handleGetDatacenter)
	handleRead("GET /grappa-dcim/v1/facilities/datacenters/{id}/map", h.handleGetDatacenterMap)
	handleRead("GET /grappa-dcim/v1/facilities/datacenters/{id}/layout-grid", h.handleGetDatacenterLayoutGrid)
	handleRead("GET /grappa-dcim/v1/facilities/datacenters/{id}/ports", h.handleListDatacenterPorts)
	handleRead("GET /grappa-dcim/v1/layout/islets", h.handleListIslets)
	handleRead("GET /grappa-dcim/v1/layout/islets/{id}/positions", h.handleListPositions)
	handleRead("GET /grappa-dcim/v1/racks", h.handleListRacks)
	handleRead("GET /grappa-dcim/v1/racks/{id}", h.handleGetRack)
	handleRead("GET /grappa-dcim/v1/racks/{id}/units", h.handleListRackUnits)
	handleRead("GET /grappa-dcim/v1/racks/{id}/media", h.handleListRackMedia)
	handleRead("GET /grappa-dcim/v1/racks/{id}/sockets", h.handleListRackSockets)
	handleRead("GET /grappa-dcim/v1/racks/{id}/power-readings", h.handleListRackPowerReadings)
	handleRead("GET /grappa-dcim/v1/racks/{id}/power-summary", h.handleRackPowerSummary)
	handleRead("GET /grappa-dcim/v1/equipment", h.handleListEquipment)
	handleRead("GET /grappa-dcim/v1/equipment/type-options", h.handleEquipmentTypeOptions)
	handleRead("GET /grappa-dcim/v1/equipment/{id}", h.handleGetEquipment)
	handleRead("GET /grappa-dcim/v1/equipment/{id}/nics", h.handleListEquipmentNICs)
	handleRead("GET /grappa-dcim/v1/servers", h.handleListServers)
	handleRead("GET /grappa-dcim/v1/servers/{id}", h.handleGetServer)
	handleRead("GET /grappa-dcim/v1/servers/{id}/children", h.handleGetServerChildren)
	handleRead("GET /grappa-dcim/v1/storage", h.handleListStorage)
	handleRead("GET /grappa-dcim/v1/storage/{id}", h.handleGetStorage)
	handleRead("GET /grappa-dcim/v1/cameras", h.handleListCameras)
	handleRead("GET /grappa-dcim/v1/cameras/{id}", h.handleGetCamera)
	handleRead("GET /grappa-dcim/v1/plenums", h.handleListPlenums)
	handleRead("GET /grappa-dcim/v1/plenums/{id}", h.handleGetPlenum)
	handleRead("GET /grappa-dcim/v1/plenums/{id}/matrix", h.handleGetPlenumMatrix)
	handleRead("GET /grappa-dcim/v1/cables", h.handleListCables)
	handleRead("GET /grappa-dcim/v1/cables/{id}", h.handleGetCable)
	handleRead("GET /grappa-dcim/v1/cables/{id}/fibers", h.handleListCableFibers)
	handleRead("GET /grappa-dcim/v1/ports", h.handleListPorts)
	handleRead("GET /grappa-dcim/v1/xcon", h.handleListXcon)
	handleRead("GET /grappa-dcim/v1/xcon/product-options", h.handleXconProductOptions)
	handleRead("GET /grappa-dcim/v1/xcon/{id}", h.handleGetXcon)
	handleRead("GET /grappa-dcim/v1/fiber-rings", h.handleListFiberRings)
	handleRead("GET /grappa-dcim/v1/fiber-rings/{id}", h.handleGetFiberRing)
	handleRead("GET /grappa-dcim/v1/fiber-rings/{id}/topology", h.handleGetFiberRingTopology)
	handleRead("GET /grappa-dcim/v1/fiber-rings/{id}/kml", h.handleGetFiberRingKML)
	handleRead("GET /grappa-dcim/v1/artifacts/{artifactId}/download", h.handleDownloadArtifact)

	writeProtect := RequireOperativo
	handleWrite := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, writeProtect(http.HandlerFunc(handler)))
	}
	handleCredential := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, writeProtect(http.HandlerFunc(handler)))
	}
	handleWrite("POST /grappa-dcim/v1/facilities/buildings", h.handleCreateBuilding)
	handleWrite("PATCH /grappa-dcim/v1/facilities/buildings/{id}", h.handleUpdateBuilding)
	handleWrite("POST /grappa-dcim/v1/facilities/buildings/{id}/cease", h.handleCeaseBuilding)
	handleWrite("DELETE /grappa-dcim/v1/facilities/buildings/{id}", h.handleDeleteBuilding)
	handleWrite("POST /grappa-dcim/v1/facilities/datacenters", h.handleCreateDatacenter)
	handleWrite("PATCH /grappa-dcim/v1/facilities/datacenters/{id}", h.handleUpdateDatacenter)
	handleWrite("POST /grappa-dcim/v1/facilities/datacenters/{id}/cease", h.handleCeaseDatacenter)
	handleWrite("DELETE /grappa-dcim/v1/facilities/datacenters/{id}", h.handleDeleteDatacenter)
	handleWrite("POST /grappa-dcim/v1/facilities/datacenters/{id}/ports", h.handleCreateDatacenterPort)
	handleWrite("POST /grappa-dcim/v1/layout/islets", h.handleCreateIslet)
	handleWrite("PATCH /grappa-dcim/v1/layout/islets/{id}", h.handleUpdateIslet)
	handleWrite("DELETE /grappa-dcim/v1/layout/islets/{id}", h.handleDeleteIslet)
	handleWrite("POST /grappa-dcim/v1/layout/islets/{id}/positions/batch", h.handleCreatePositionBatch)
	handleWrite("PATCH /grappa-dcim/v1/layout/positions/{id}", h.handleUpdatePosition)
	handleWrite("DELETE /grappa-dcim/v1/layout/positions/{id}", h.handleDeletePosition)
	handleWrite("POST /grappa-dcim/v1/racks", h.handleCreateRack)
	handleWrite("PATCH /grappa-dcim/v1/racks/{id}", h.handleUpdateRack)
	handleWrite("POST /grappa-dcim/v1/racks/{id}/move", h.handleMoveRack)
	handleWrite("POST /grappa-dcim/v1/racks/{id}/cease", h.handleCeaseRack)
	handleWrite("DELETE /grappa-dcim/v1/racks/{id}", h.handleDeleteRack)
	handleWrite("PUT /grappa-dcim/v1/racks/{id}/media", h.handleReplaceRackMedia)
	handleWrite("POST /grappa-dcim/v1/racks/{id}/sockets", h.handleCreateRackSocket)
	handleWrite("PATCH /grappa-dcim/v1/rack-sockets/{socketId}", h.handleUpdateRackSocket)
	handleWrite("DELETE /grappa-dcim/v1/rack-sockets/{socketId}", h.handleDeleteRackSocket)
	handleWrite("POST /grappa-dcim/v1/equipment", h.handleCreateEquipment)
	handleWrite("PATCH /grappa-dcim/v1/equipment/{id}", h.handleUpdateEquipment)
	handleWrite("POST /grappa-dcim/v1/equipment/{id}/cease", h.handleCeaseEquipment)
	handleWrite("POST /grappa-dcim/v1/servers", h.handleCreateServer)
	handleWrite("PATCH /grappa-dcim/v1/servers/{id}", h.handleUpdateServer)
	handleCredential("GET /grappa-dcim/v1/servers/{id}/credentials", h.handleGetServerCredentials)
	handleCredential("PATCH /grappa-dcim/v1/servers/{id}/credentials", h.handleUpdateServerCredentials)
	handleWrite("POST /grappa-dcim/v1/storage", h.handleCreateStorage)
	handleWrite("PATCH /grappa-dcim/v1/storage/{id}", h.handleUpdateStorage)
	handleWrite("POST /grappa-dcim/v1/storage/{id}/archive", h.handleArchiveStorage)
	handleWrite("DELETE /grappa-dcim/v1/storage/{id}", h.handleDeleteStorage)
	handleWrite("POST /grappa-dcim/v1/cameras", h.handleCreateCamera)
	handleWrite("PATCH /grappa-dcim/v1/cameras/{id}", h.handleUpdateCamera)
	handleWrite("POST /grappa-dcim/v1/plenums", h.handleCreatePlenum)
	handleWrite("PATCH /grappa-dcim/v1/plenums/{id}", h.handleUpdatePlenum)
	handleWrite("DELETE /grappa-dcim/v1/plenums/{id}", h.handleDeletePlenum)
	handleWrite("POST /grappa-dcim/v1/plenums/{id}/initialize-matrix", h.handleInitializePlenumMatrix)
	handleWrite("POST /grappa-dcim/v1/cables", h.handleCreateCable)
	handleWrite("PATCH /grappa-dcim/v1/cables/{id}", h.handleUpdateCable)
	handleWrite("DELETE /grappa-dcim/v1/cables/{id}", h.handleDeleteCable)
	handleWrite("PATCH /grappa-dcim/v1/fibers/{id}/assignment", h.handleAssignFiber)
	handleWrite("POST /grappa-dcim/v1/xcon", h.handleCreateXcon)
	handleWrite("PATCH /grappa-dcim/v1/xcon/{id}", h.handleUpdateXcon)
	handleWrite("PUT /grappa-dcim/v1/xcon/{id}/hops", h.handleReplaceXconHops)
	handleWrite("POST /grappa-dcim/v1/fiber-rings", h.handleCreateFiberRing)
	handleWrite("PATCH /grappa-dcim/v1/fiber-rings/{id}", h.handleUpdateFiberRing)
	handleWrite("POST /grappa-dcim/v1/fiber-rings/{id}/increase-nodes", h.handleIncreaseFiberRingNodes)
	handleWrite("POST /grappa-dcim/v1/fiber-rings/{id}/cease", h.handleCeaseFiberRing)
	handleWrite("DELETE /grappa-dcim/v1/fiber-rings/{id}", h.handleDeleteFiberRing)
	handleWrite("PATCH /grappa-dcim/v1/fiber-rings/{id}/nodes/{nodeId}", h.handleUpdateFiberRingNode)
	handleWrite("PATCH /grappa-dcim/v1/fiber-rings/{id}/arcs/{arcId}", h.handleUpdateFiberRingArc)
	handleWrite("PUT /grappa-dcim/v1/fiber-rings/{id}/routes", h.handleReplaceFiberRingRoutes)
	handleWrite("POST /grappa-dcim/v1/fiber-rings/{id}/kml", h.handleUploadFiberRingKML)
}

func RequireOperativo(next http.Handler) http.Handler {
	return acl.RequireRole(applaunch.GrappaDCIMOperativoRoles()...)(next)
}

func (h *Handler) requireDB(w http.ResponseWriter) bool {
	if h.grappa == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "grappa_dcim_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", component, "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func (h *Handler) rowsDone(w http.ResponseWriter, r *http.Request, rows *sql.Rows, operation string) bool {
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, operation+"_rows", err)
		return false
	}
	return true
}

func (h *Handler) handleMeta(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	claims, _ := auth.GetClaims(r.Context())
	canOperate := authz.HasAnyRole(claims.Roles, applaunch.GrappaDCIMOperativoRoles()...)
	httputil.JSON(w, http.StatusOK, MetaResponse{
		CanRead:            true,
		CanOperate:         canOperate,
		CanViewCredentials: canOperate,
		AppVersion:         h.appVersion,
	})
}

func (h *Handler) handleLookups(w http.ResponseWriter, _ *http.Request) {
	if !h.requireDB(w) {
		return
	}

	httputil.JSON(w, http.StatusOK, LookupResponse{
		Infrastructure: []LookupItem{},
		Assets:         []LookupItem{},
		Connectivity:   []LookupItem{},
		Topology:       []LookupItem{},
	})
}
