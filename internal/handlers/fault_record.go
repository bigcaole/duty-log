package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/middleware"
	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type faultRecordListItem struct {
	ID                 uint
	Date               string
	DutyPerson         string
	UserName           string
	FaultTypeName      string
	Status             string
	ProcessingStatus   string
	ReceivedTime       string
	CompletedTime      string
	ProcessingDuration string
}

type faultRecordFormView struct {
	ID                    uint
	Date                  string
	DutyPerson            string
	Status                string
	UserName              string
	ReceivedTime          string
	FaultTypeID           string
	FaultSymptom          string
	ProcessingProcess     string
	CompletedTime         string
	CustomerServicePerson string
	ProcessingStatus      string
	Remarks               string
	ReminderEnabled       bool
	ReminderDate          string
	ReminderTitle         string
	ReminderContent       string
	ReminderDaysBefore    string
}

func registerFaultRecordRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/fault-records", app.faultRecordList)
	group.GET("/fault-records/create", app.faultRecordCreatePage)
	group.POST("/fault-records/create", app.faultRecordCreate)
	group.GET("/fault-records/:id", app.faultRecordDetail)
	group.GET("/fault-records/:id/edit", app.faultRecordEditPage)
	group.POST("/fault-records/:id/edit", app.faultRecordUpdate)
	group.POST("/fault-records/:id/delete", app.faultRecordDelete)
}

func (a *AppContext) faultRecordList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	dateFrom := strings.TrimSpace(c.Query("date_from"))
	dateTo := strings.TrimSpace(c.Query("date_to"))
	status := strings.TrimSpace(c.Query("status"))
	processingStatus := strings.TrimSpace(c.Query("processing_status"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	var records []models.FaultRecord
	query := a.DB.Order("date desc, id desc")
	if !currentUser.IsAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	}
	if parsed, err := parseOptionalDate(dateFrom); err == nil && parsed != nil {
		query = query.Where("date >= ?", parsed.Format(dateLayout))
	}
	if parsed, err := parseOptionalDate(dateTo); err == nil && parsed != nil {
		query = query.Where("date <= ?", parsed.Format(dateLayout))
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if processingStatus != "" {
		query = query.Where("processing_status = ?", processingStatus)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"duty_person ILIKE ? OR user_name ILIKE ? OR fault_symptom ILIKE ? OR processing_process ILIKE ? OR remarks ILIKE ?",
			like, like, like, like, like,
		)
	}
	if err := query.Find(&records).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "网络故障记录",
			"Path":    "/fault-records",
			"Message": "读取网络故障记录失败：" + err.Error(),
		})
		return
	}

	var faultTypes []models.FaultType
	_ = a.DB.Order("name asc").Find(&faultTypes).Error
	typeNameByID := make(map[uint]string, len(faultTypes))
	for _, item := range faultTypes {
		typeNameByID[item.ID] = item.Name
	}

	now := time.Now()
	items := make([]faultRecordListItem, 0, len(records))
	for _, record := range records {
		typeName := "-"
		if name, ok := typeNameByID[record.FaultTypeID]; ok {
			typeName = name
		}
		completedText := "-"
		durationText := "-"
		if record.CompletedTime != nil {
			completedText = record.CompletedTime.Format("2006-01-02 15:04")
			durationText = formatDuration(record.CompletedTime.Sub(record.ReceivedTime))
		} else {
			durationText = formatDuration(now.Sub(record.ReceivedTime)) + "（进行中）"
		}
		items = append(items, faultRecordListItem{
			ID:                 record.ID,
			Date:               record.Date.Format(dateLayout),
			DutyPerson:         record.DutyPerson,
			UserName:           record.UserName,
			FaultTypeName:      typeName,
			Status:             record.Status,
			ProcessingStatus:   record.ProcessingStatus,
			ReceivedTime:       record.ReceivedTime.Format("2006-01-02 15:04"),
			CompletedTime:      completedText,
			ProcessingDuration: durationText,
		})
	}

	c.HTML(http.StatusOK, "fault_record/list.html", gin.H{
		"Title":   "网络故障记录",
		"Items":   items,
		"IsAdmin": currentUser.IsAdmin,
		"Msg":     strings.TrimSpace(c.Query("msg")),
		"Error":   strings.TrimSpace(c.Query("error")),
		"Filter": gin.H{
			"DateFrom":         dateFrom,
			"DateTo":           dateTo,
			"Status":           status,
			"ProcessingStatus": processingStatus,
			"Keyword":          keyword,
		},
	})
}

func (a *AppContext) faultRecordCreatePage(c *gin.Context) {
	a.renderFaultRecordForm(c, http.StatusOK, "新建网络故障记录", "/fault-records/create", faultRecordFormView{
		Date:             todayDateString(),
		Status:           "normal",
		ReceivedTime:     nowDateTimeLocalString(),
		ProcessingStatus: "pending",
	}, "")
}

func (a *AppContext) faultRecordCreate(c *gin.Context) {
	record, formView, err := a.bindFaultRecordForm(c)
	if err != nil {
		a.renderFaultRecordForm(c, http.StatusBadRequest, "新建网络故障记录", "/fault-records/create", formView, err.Error())
		return
	}

	userID, ok := middleware.CurrentUserID(c)
	if !ok || userID == 0 {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	record.UserID = userID

	reminderReq := readReminderRequest(c)
	formView.ReminderEnabled = reminderReq.Enabled
	formView.ReminderDate = reminderReq.Date
	formView.ReminderTitle = reminderReq.Title
	formView.ReminderContent = reminderReq.Content
	formView.ReminderDaysBefore = reminderReq.DaysBefore

	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		reminder, err := buildReminderFromRequest(
			reminderReq,
			record.Date,
			userID,
			fmt.Sprintf("网络故障提醒：%s", record.UserName),
			fmt.Sprintf("故障现象: %s\n处理状态: %s", record.FaultSymptom, record.ProcessingStatus),
		)
		if err != nil {
			return err
		}
		if reminder != nil {
			if err := tx.Create(reminder).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		a.renderFaultRecordForm(c, http.StatusBadRequest, "新建网络故障记录", "/fault-records/create", formView, "创建失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/fault-records?msg=创建成功")
}

func (a *AppContext) faultRecordDetail(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/fault-records?error=无效记录ID")
		return
	}

	var record models.FaultRecord
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/fault-records?error=记录不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, record.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/fault-records?error=无权查看他人记录")
		return
	}

	var typeName string
	var faultType models.FaultType
	if err := a.DB.First(&faultType, record.FaultTypeID).Error; err == nil {
		typeName = faultType.Name
	}

	c.HTML(http.StatusOK, "fault_record/detail.html", gin.H{
		"Title":    "网络故障记录预览",
		"Record":   record,
		"TypeName": typeName,
	})
}

func (a *AppContext) faultRecordEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/fault-records?error=无效记录ID")
		return
	}

	var record models.FaultRecord
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/fault-records?error=记录不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, record.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/fault-records?error=无权编辑他人记录")
		return
	}

	formView := faultRecordFormView{
		ID:                    record.ID,
		Date:                  record.Date.Format(dateLayout),
		DutyPerson:            record.DutyPerson,
		Status:                record.Status,
		UserName:              record.UserName,
		ReceivedTime:          record.ReceivedTime.Format(dateTimeLocalLayout),
		FaultTypeID:           strconv.FormatUint(uint64(record.FaultTypeID), 10),
		FaultSymptom:          record.FaultSymptom,
		ProcessingProcess:     record.ProcessingProcess,
		CustomerServicePerson: record.CustomerServicePerson,
		ProcessingStatus:      record.ProcessingStatus,
		Remarks:               record.Remarks,
	}
	if record.CompletedTime != nil {
		formView.CompletedTime = record.CompletedTime.Format(dateTimeLocalLayout)
	}

	a.renderFaultRecordForm(c, http.StatusOK, "编辑网络故障记录", "/fault-records/"+strconv.FormatUint(id, 10)+"/edit", formView, "")
}

func (a *AppContext) faultRecordUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/fault-records?error=无效记录ID")
		return
	}

	var existing models.FaultRecord
	if err := a.DB.First(&existing, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/fault-records?error=记录不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, existing.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/fault-records?error=无权编辑他人记录")
		return
	}

	record, formView, bindErr := a.bindFaultRecordForm(c)
	formView.ID = existing.ID
	if bindErr != nil {
		a.renderFaultRecordForm(c, http.StatusBadRequest, "编辑网络故障记录", "/fault-records/"+strconv.FormatUint(id, 10)+"/edit", formView, bindErr.Error())
		return
	}

	existing.Date = record.Date
	existing.DutyPerson = record.DutyPerson
	existing.Status = record.Status
	existing.UserName = record.UserName
	existing.ReceivedTime = record.ReceivedTime
	existing.FaultTypeID = record.FaultTypeID
	existing.FaultSymptom = record.FaultSymptom
	existing.ProcessingProcess = record.ProcessingProcess
	existing.CompletedTime = record.CompletedTime
	existing.CustomerServicePerson = record.CustomerServicePerson
	existing.ProcessingStatus = record.ProcessingStatus
	existing.Remarks = record.Remarks
	existing.UpdatedAt = time.Now()

	reminderReq := readReminderRequest(c)
	formView.ReminderEnabled = reminderReq.Enabled
	formView.ReminderDate = reminderReq.Date
	formView.ReminderTitle = reminderReq.Title
	formView.ReminderContent = reminderReq.Content
	formView.ReminderDaysBefore = reminderReq.DaysBefore

	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
		reminder, err := buildReminderFromRequest(
			reminderReq,
			existing.Date,
			currentUser.ID,
			fmt.Sprintf("网络故障提醒：%s", existing.UserName),
			fmt.Sprintf("故障现象: %s\n处理状态: %s", existing.FaultSymptom, existing.ProcessingStatus),
		)
		if err != nil {
			return err
		}
		if reminder != nil {
			if err := tx.Create(reminder).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		a.renderFaultRecordForm(c, http.StatusBadRequest, "编辑网络故障记录", "/fault-records/"+strconv.FormatUint(id, 10)+"/edit", formView, "更新失败："+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/fault-records?msg=更新成功")
}

func (a *AppContext) faultRecordDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/fault-records?error=无效记录ID")
		return
	}
	var record models.FaultRecord
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/fault-records?error=记录不存在")
		return
	}
	if !canAccessOwnedRecord(currentUser.IsAdmin, record.UserID, currentUser.ID) {
		c.Redirect(http.StatusFound, "/fault-records?error=无权删除他人记录")
		return
	}
	if err := a.DB.Delete(&models.FaultRecord{}, record.ID).Error; err != nil {
		c.Redirect(http.StatusFound, "/fault-records?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/fault-records?msg=删除成功")
}

func (a *AppContext) bindFaultRecordForm(c *gin.Context) (models.FaultRecord, faultRecordFormView, error) {
	reminderReq := readReminderRequest(c)
	formView := faultRecordFormView{
		Date:                  strings.TrimSpace(c.PostForm("date")),
		DutyPerson:            strings.TrimSpace(c.PostForm("duty_person")),
		Status:                strings.TrimSpace(c.PostForm("status")),
		UserName:              strings.TrimSpace(c.PostForm("user_name")),
		ReceivedTime:          strings.TrimSpace(c.PostForm("received_time")),
		FaultTypeID:           strings.TrimSpace(c.PostForm("fault_type_id")),
		FaultSymptom:          strings.TrimSpace(c.PostForm("fault_symptom")),
		ProcessingProcess:     strings.TrimSpace(c.PostForm("processing_process")),
		CompletedTime:         strings.TrimSpace(c.PostForm("completed_time")),
		CustomerServicePerson: strings.TrimSpace(c.PostForm("customer_service_person")),
		ProcessingStatus:      strings.TrimSpace(c.PostForm("processing_status")),
		Remarks:               strings.TrimSpace(c.PostForm("remarks")),
		ReminderEnabled:       reminderReq.Enabled,
		ReminderDate:          reminderReq.Date,
		ReminderTitle:         reminderReq.Title,
		ReminderContent:       reminderReq.Content,
		ReminderDaysBefore:    reminderReq.DaysBefore,
	}

	date, err := parseRequiredDate(formView.Date)
	if err != nil {
		return models.FaultRecord{}, formView, err
	}
	if formView.DutyPerson == "" {
		return models.FaultRecord{}, formView, fmt.Errorf("值班人员不能为空")
	}
	if formView.Status == "" {
		formView.Status = "normal"
	}
	if formView.UserName == "" {
		return models.FaultRecord{}, formView, fmt.Errorf("用户名称不能为空")
	}
	receivedTime, err := parseRequiredDateTime(formView.ReceivedTime)
	if err != nil {
		return models.FaultRecord{}, formView, err
	}
	faultTypeID, err := parseRequiredUint(formView.FaultTypeID, "故障类型")
	if err != nil {
		return models.FaultRecord{}, formView, err
	}
	if formView.FaultSymptom == "" {
		return models.FaultRecord{}, formView, fmt.Errorf("故障现象不能为空")
	}
	if formView.ProcessingProcess == "" {
		return models.FaultRecord{}, formView, fmt.Errorf("处理过程不能为空")
	}

	completedTime, err := parseOptionalDateTime(formView.CompletedTime)
	if err != nil {
		return models.FaultRecord{}, formView, err
	}
	if completedTime != nil && completedTime.Before(receivedTime) {
		return models.FaultRecord{}, formView, fmt.Errorf("完成时间不能早于受理时间")
	}
	if formView.ProcessingStatus == "" {
		formView.ProcessingStatus = "pending"
	}

	record := models.FaultRecord{
		Date:                  date,
		DutyPerson:            formView.DutyPerson,
		Status:                formView.Status,
		UserName:              formView.UserName,
		ReceivedTime:          receivedTime,
		FaultTypeID:           faultTypeID,
		FaultSymptom:          formView.FaultSymptom,
		ProcessingProcess:     formView.ProcessingProcess,
		CompletedTime:         completedTime,
		CustomerServicePerson: formView.CustomerServicePerson,
		ProcessingStatus:      formView.ProcessingStatus,
		Remarks:               formView.Remarks,
	}
	return record, formView, nil
}

func (a *AppContext) renderFaultRecordForm(c *gin.Context, statusCode int, title, action string, formView faultRecordFormView, errorMessage string) {
	var faultTypes []models.FaultType
	_ = a.DB.Order("name asc").Find(&faultTypes).Error
	if strings.TrimSpace(formView.ReminderDaysBefore) == "" {
		formView.ReminderDaysBefore = "2"
	}

	c.HTML(statusCode, "fault_record/form.html", gin.H{
		"Title":            title,
		"Action":           action,
		"Form":             formView,
		"FaultTypes":       faultTypes,
		"Error":            errorMessage,
		"Statuses":         []string{"normal", "warning", "critical"},
		"ProcessingStatus": []string{"pending", "processing", "completed"},
	})
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = -duration
	}
	totalMinutes := int(duration.Minutes())
	days := totalMinutes / (24 * 60)
	hours := (totalMinutes % (24 * 60)) / 60
	minutes := totalMinutes % 60
	if days > 0 {
		return fmt.Sprintf("%d天 %d小时 %d分钟", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时 %d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", minutes)
}
