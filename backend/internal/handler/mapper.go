package handler

import (
	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/dto"
	"github.com/labequipment/lab-equipment/internal/model"
)

func mapUser(user model.User) dto.UserResponse {
	roleCode := ""
	roleName := ""
	if user.Role != nil {
		roleCode = user.Role.Code
		roleName = user.Role.Name
	}
	return dto.UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Name:     user.Name,
		Email:    user.Email,
		Phone:    user.Phone,
		RoleID:   user.RoleID,
		RoleCode: roleCode,
		RoleName: roleName,
	}
}

func mapEquipment(equipment model.Equipment) dto.EquipmentResponse {
	categoryName := ""
	if equipment.Category != nil {
		categoryName = equipment.Category.Name
	}
	ownerName := ""
	if equipment.Owner != nil {
		ownerName = equipment.Owner.Name
	}
	return dto.EquipmentResponse{
		ID:             equipment.ID,
		Name:           equipment.Name,
		Code:           equipment.Code,
		CategoryID:     equipment.CategoryID,
		CategoryName:   categoryName,
		BrandModel:     equipment.BrandModel,
		SerialNumber:   equipment.SerialNumber,
		PurchaseDate:   equipment.PurchaseDate,
		PurchasePrice:  equipment.PurchasePrice,
		Location:       equipment.Location,
		Status:         string(equipment.Status),
		OwnerID:        equipment.OwnerID,
		OwnerName:      ownerName,
		Supplier:       equipment.Supplier,
		WarrantyExpiry: equipment.WarrantyExpiry,
		ImageURL:       equipment.ImageURL,
		CreatedAt:      equipment.CreatedAt,
		UpdatedAt:      equipment.UpdatedAt,
	}
}

func mapBorrow(record model.BorrowRecord) dto.BorrowResponse {
	equipmentName := ""
	equipmentCode := ""
	if record.Equipment != nil {
		equipmentName = record.Equipment.Name
		equipmentCode = record.Equipment.Code
	}
	borrowerName := ""
	if record.Borrower != nil {
		borrowerName = record.Borrower.Name
	}
	approverName := ""
	if record.Approver != nil {
		approverName = record.Approver.Name
	}
	var returnCondition *string
	if record.ReturnCondition != nil {
		value := string(*record.ReturnCondition)
		returnCondition = &value
	}
	var pendingRenewal *dto.PendingRenewalBrief
	for i := range record.Renewals {
		if record.Renewals[i].Status == constants.RenewalStatusPending {
			item := record.Renewals[i]
			pendingRenewal = &dto.PendingRenewalBrief{
				ID:               item.ID,
				ExtendDays:       item.ExtendDays,
				RequestedDueDate: item.RequestedDueDate,
				CreatedAt:        item.CreatedAt,
			}
			break
		}
	}
	originalExpectedReturn := record.OriginalExpectedReturn
	if originalExpectedReturn.IsZero() {
		originalExpectedReturn = record.ExpectedReturnDate
	}
	return dto.BorrowResponse{
		ID:                     record.ID,
		EquipmentID:            record.EquipmentID,
		EquipmentName:          equipmentName,
		EquipmentCode:          equipmentCode,
		BorrowerID:             record.BorrowerID,
		BorrowerName:           borrowerName,
		BorrowDate:             record.BorrowDate,
		ExpectedReturnDate:     record.ExpectedReturnDate,
		OriginalExpectedReturn: originalExpectedReturn,
		ActualReturnDate:       record.ActualReturnDate,
		Reason:                 record.Reason,
		Status:                 string(record.Status),
		ApproverID:             record.ApproverID,
		ApproverName:           approverName,
		ReturnCondition:        returnCondition,
		PendingRenewal:         pendingRenewal,
		CreatedAt:              record.CreatedAt,
	}
}

func mapRenewal(renewal model.BorrowRenewal) dto.RenewalResponse {
	equipmentID := uint(0)
	equipmentName := ""
	if renewal.Borrow != nil && renewal.Borrow.Equipment != nil {
		equipmentID = renewal.Borrow.Equipment.ID
		equipmentName = renewal.Borrow.Equipment.Name
	}
	applicantName := ""
	if renewal.Applicant != nil {
		applicantName = renewal.Applicant.Name
	}
	reviewerName := ""
	if renewal.Reviewer != nil {
		reviewerName = renewal.Reviewer.Name
	}
	return dto.RenewalResponse{
		ID:               renewal.ID,
		BorrowID:         renewal.BorrowID,
		EquipmentID:      equipmentID,
		EquipmentName:    equipmentName,
		ApplicantID:      renewal.ApplicantID,
		ApplicantName:    applicantName,
		ExtendDays:       renewal.ExtendDays,
		CurrentDueDate:   renewal.CurrentDueDate,
		RequestedDueDate: renewal.RequestedDueDate,
		Reason:           renewal.Reason,
		Status:           string(renewal.Status),
		ReviewerID:       renewal.ReviewerID,
		ReviewerName:     reviewerName,
		ReviewComment:    renewal.ReviewComment,
		ReviewedAt:       renewal.ReviewedAt,
		CreatedAt:        renewal.CreatedAt,
	}
}

func mapMaintenance(record model.MaintenanceRecord) dto.MaintenanceResponse {
	equipmentName := ""
	if record.Equipment != nil {
		equipmentName = record.Equipment.Name
	}
	maintainerName := ""
	if record.Maintainer != nil {
		maintainerName = record.Maintainer.Name
	}
	return dto.MaintenanceResponse{
		ID:                  record.ID,
		EquipmentID:         record.EquipmentID,
		EquipmentName:       equipmentName,
		Type:                string(record.Type),
		Content:             record.Content,
		MaintenanceDate:     record.MaintenanceDate,
		NextMaintenanceDate: record.NextMaintenanceDate,
		Cost:                record.Cost,
		MaintainerID:        record.MaintainerID,
		MaintainerName:      maintainerName,
		Result:              string(record.Result),
		CreatedAt:           record.CreatedAt,
	}
}

func mapReservation(reservation model.Reservation) dto.ReservationResponse {
	equipmentName := ""
	if reservation.Equipment != nil {
		equipmentName = reservation.Equipment.Name
	}
	userName := ""
	if reservation.User != nil {
		userName = reservation.User.Name
	}
	approverName := ""
	if reservation.Approver != nil {
		approverName = reservation.Approver.Name
	}
	return dto.ReservationResponse{
		ID:            reservation.ID,
		EquipmentID:   reservation.EquipmentID,
		EquipmentName: equipmentName,
		UserID:        reservation.UserID,
		UserName:      userName,
		StartTime:     reservation.StartTime,
		EndTime:       reservation.EndTime,
		Purpose:       reservation.Purpose,
		Status:        string(reservation.Status),
		ApproverID:    reservation.ApproverID,
		ApproverName:  approverName,
		CreatedAt:     reservation.CreatedAt,
	}
}

func mapCategory(category model.EquipmentCategory) dto.CategoryResponse {
	children := make([]dto.CategoryResponse, 0, len(category.Children))
	for _, child := range category.Children {
		children = append(children, mapCategory(child))
	}
	return dto.CategoryResponse{
		ID:          category.ID,
		Name:        category.Name,
		ParentID:    category.ParentID,
		Description: category.Description,
		Icon:        category.Icon,
		Children:    children,
	}
}

func mapAudit(log model.AuditLog) dto.AuditLogResponse {
	return dto.AuditLogResponse{
		ID:           log.ID,
		UserID:       log.UserID,
		UserName:     log.UserName,
		Action:       log.Action,
		ResourceType: log.ResourceType,
		ResourceID:   log.ResourceID,
		Detail:       log.Detail,
		IP:           log.IP,
		CreatedAt:    log.CreatedAt,
	}
}
