package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/middleware"
	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
)

type ipamSectionItem struct {
	ID              uint
	Name            string
	RootCIDRs       []string
	RootCount       int
	SubnetCount     int
	IPv4Used        int64
	IPv4Total       int64
	IPv6Used        int64
	MaxFreePrefix   string
	MaxFreeHint     string
	UtilPercent     int
	UtilPercentText string
}

type ipamSearchSubnet struct {
	ID           uint
	Network      string
	Unit         string
	Section      string
	RateMbps     int
	VlanID       int
	L2Port       string
	EgressDevice string
	Description  string
	UpdatedAt    string
	UsedCount    int64
	TotalCount   string
	UtilPercent  string
}

type ipamSearchAddress struct {
	ID           uint
	IP           string
	Status       string
	Unit         string
	Hostname     string
	Note         string
	SubnetID     uint
	Network      string
	Section      string
	RateMbps     int
	VlanID       int
	L2Port       string
	EgressDevice string
	UpdatedAt    string
}

type ipamSectionForm struct {
	ID       uint
	Name     string
	RootCIDR string
}

type ipamCategoryRootItem struct {
	ID   uint
	CIDR string
	Note string
}

type ipamSubnetItem struct {
	ID           uint
	ParentID     uint
	Network      string
	Unit         string
	RateMbps     int
	VlanID       int
	L2Port       string
	EgressDevice string
	UpdatedAt    string
	UsedCount    int64
	TotalCount   string
	UtilPercent  string
	Depth        int
	IsIPv6       bool
	HasChildren  bool
}

type ipamSubnetForm struct {
	ID           uint
	SectionID    uint
	ParentID     string
	Network      string
	Unit         string
	RateMbps     string
	VlanID       string
	L2Port       string
	EgressDevice string
	Description  string
}

type ipamAddressItem struct {
	ID        uint
	IP        string
	Status    string
	Unit      string
	Hostname  string
	Note      string
	UpdatedAt string
}

type ipamAddressForm struct {
	ID       uint
	SubnetID uint
	IP       string
	Status   string
	Unit     string
	Hostname string
	Note     string
}

type ipamSplitForm struct {
	SubnetID  uint
	Network   string
	NewPrefix string
	Count     int
}

type ipamImportResult struct {
	Total   int
	Valid   int
	Invalid int
	Created int
	Updated int
	Skipped int
	Errors  []string
	Samples []ipamImportRow
}

type ipamImportRow struct {
	IP       string
	Status   string
	Unit     string
	Hostname string
	Note     string
	Error    string
}

type ipamSubnetImportResult struct {
	Total   int
	Valid   int
	Invalid int
	Created int
	Updated int
	Deleted int
	Errors  []string
	Samples []ipamSubnetImportRow
}

type ipamSubnetImportRow struct {
	RootCIDR    string
	Subnet      string
	Unit        string
	RateMbps    string
	VlanID      string
	L2Port      string
	Device      string
	Description string
	Error       string
}

type ipamGenerateForm struct {
	SubnetID       uint
	StartIP        string
	EndIP          string
	Count          string
	Status         string
	Unit           string
	HostnamePrefix string
	Note           string
}

type ipamGenerateResult struct {
	Total   int
	Valid   int
	Created int
	Skipped int
	Samples []string
	Errors  []string
}

type ipamBlockCell struct {
	IP     string
	Label  string
	Class  string
	Status string
}

type ipamBlockMap struct {
	Columns  int
	Cells    []ipamBlockCell
	Note     string
	Page     int
	Total    int64
	Pages    int
	Range    string
	PrevPage int
	NextPage int
	HasPrev  bool
	HasNext  bool
}

type ipamMapBlock struct {
	CIDR string
	Used bool
	Link string
}

type ipamMapRow struct {
	PrefixLen int
	Used      int
	Free      int
	Total     int
	Blocks    []ipamMapBlock
}

type ipamMapSpace struct {
	Enabled       bool
	Rows          []ipamMapRow
	UtilPercent   int
	UtilText      string
	UsedAddresses int64
	TotalText     string
	Note          string
}

var ipamStatusOptions = []statusOption{
	{Value: "used", Label: "已使用"},
	{Value: "reserved", Label: "保留"},
	{Value: "dhcp", Label: "DHCP"},
	{Value: "free", Label: "空闲"},
}

func registerIPAMRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/ipam", app.ipamSectionList)
	group.GET("/ipam/sections/create", app.ipamSectionCreatePage)
	group.POST("/ipam/sections/create", app.ipamSectionCreate)
	group.GET("/ipam/sections/:id", app.ipamSectionDetail)
	group.GET("/ipam/sections/:id/import", app.ipamSectionImportPage)
	group.POST("/ipam/sections/:id/import", app.ipamSectionImport)
	group.GET("/ipam/sections/:id/import/template", app.ipamSectionImportTemplate)
	group.POST("/ipam/sections/:id/roots", app.ipamSectionRootAdd)
	group.POST("/ipam/sections/:id/roots/:rootID/delete", app.ipamSectionRootDelete)
	group.GET("/ipam/sections/:id/edit", app.ipamSectionEditPage)
	group.POST("/ipam/sections/:id/edit", app.ipamSectionUpdate)
	group.POST("/ipam/sections/:id/delete", app.ipamSectionDelete)

	group.GET("/ipam/subnets/create", app.ipamSubnetCreatePage)
	group.POST("/ipam/subnets/create", app.ipamSubnetCreate)
	group.GET("/ipam/subnets/virtual", app.ipamSubnetVirtualDetail)
	group.GET("/ipam/subnets/:id", app.ipamSubnetDetail)
	group.GET("/ipam/subnets/:id/edit", app.ipamSubnetEditPage)
	group.POST("/ipam/subnets/:id/edit", app.ipamSubnetUpdate)
	group.POST("/ipam/subnets/:id/delete", app.ipamSubnetDelete)
	group.GET("/ipam/subnets/:id/split", app.ipamSubnetSplitPage)
	group.POST("/ipam/subnets/:id/split", app.ipamSubnetSplit)

	group.GET("/ipam/subnets/:id/addresses", app.ipamAddressList)
	group.GET("/ipam/subnets/:id/addresses/create", app.ipamAddressCreatePage)
	group.POST("/ipam/subnets/:id/addresses/create", app.ipamAddressCreate)
	group.GET("/ipam/subnets/:id/addresses/generate", app.ipamAddressGeneratePage)
	group.POST("/ipam/subnets/:id/addresses/generate", app.ipamAddressGenerate)
	group.POST("/ipam/subnets/:id/addresses/bulk", app.ipamAddressBulkUpdate)
	group.GET("/ipam/subnets/:id/addresses/import", app.ipamAddressImportPage)
	group.POST("/ipam/subnets/:id/addresses/import", app.ipamAddressImport)
	group.GET("/ipam/subnets/:id/addresses/template", app.ipamAddressTemplate)
	group.GET("/ipam/subnets/:id/addresses/export", app.ipamAddressExport)
	group.GET("/ipam/addresses/:id/edit", app.ipamAddressEditPage)
	group.POST("/ipam/addresses/:id/edit", app.ipamAddressUpdate)
	group.POST("/ipam/addresses/:id/delete", app.ipamAddressDelete)
}

func (a *AppContext) ipamSectionList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	searchIP := strings.TrimSpace(c.Query("ip"))
	searchUnit := strings.TrimSpace(c.Query("unit"))

	sections, err := a.loadIPAMSections()
	if err != nil {
		renderIPAMError(c, "IPAM 资产管理", "/ipam", err)
		return
	}

	sectionIDs := make([]uint, 0, len(sections))
	for _, s := range sections {
		sectionIDs = append(sectionIDs, s.ID)
	}
	subnets, err := a.loadIPAMSubnetsBySection(sectionIDs)
	if err != nil {
		renderIPAMError(c, "IPAM 资产管理", "/ipam", err)
		return
	}
	addressCounts, err := a.loadIPAMUsedCountsBySubnet(subnets)
	if err != nil {
		renderIPAMError(c, "IPAM 资产管理", "/ipam", err)
		return
	}

	rootsMap, err := a.loadIPAMCategoryRoots(sectionIDs)
	if err != nil {
		renderIPAMError(c, "IPAM 资产管理", "/ipam", err)
		return
	}

	items := a.buildSectionItems(sections, subnets, addressCounts, rootsMap)
	overview := buildIPAMOverview(sections, subnets, addressCounts, rootsMap, strings.TrimSpace(c.Query("root")))

	var subnetMatches []ipamSearchSubnet
	var addressMatches []ipamSearchAddress
	if searchIP != "" || searchUnit != "" {
		subnetMatches, addressMatches = a.searchIPAM(searchIP, searchUnit)
	}

	c.HTML(http.StatusOK, "ipam/list.html", gin.H{
		"Title":           "IPAM 资产管理",
		"Sections":        items,
		"Overview":        overview,
		"CanManage":       currentUser.IsAdmin,
		"Msg":             strings.TrimSpace(c.Query("msg")),
		"Error":           strings.TrimSpace(c.Query("error")),
		"SearchIP":        searchIP,
		"SearchUnit":      searchUnit,
		"SearchSubnets":   subnetMatches,
		"SearchAddresses": addressMatches,
	})
}

func (a *AppContext) ipamSectionCreatePage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可创建类别")
		return
	}
	c.HTML(http.StatusOK, "ipam/section_form.html", gin.H{
		"Title":  "新建 IPAM 类别",
		"Action": "/ipam/sections/create",
		"Form":   ipamSectionForm{},
	})
}

func (a *AppContext) ipamSectionCreate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可创建类别")
		return
	}

	form := ipamSectionForm{
		Name:     strings.TrimSpace(c.PostForm("name")),
		RootCIDR: strings.TrimSpace(c.PostForm("root_cidr")),
	}
	if form.Name == "" {
		c.HTML(http.StatusBadRequest, "ipam/section_form.html", gin.H{
			"Title":  "新建 IPAM 类别",
			"Action": "/ipam/sections/create",
			"Form":   form,
			"Error":  "类别名称不能为空",
		})
		return
	}
	section := models.IPAMSection{
		Name: form.Name,
	}
	if err := a.DB.Create(&section).Error; err != nil {
		c.HTML(http.StatusBadRequest, "ipam/section_form.html", gin.H{
			"Title":  "新建 IPAM 类别",
			"Action": "/ipam/sections/create",
			"Form":   form,
			"Error":  "创建失败：" + err.Error(),
		})
		return
	}
	if err := a.ensureSectionRootFromForm(section.ID, form.RootCIDR); err != nil {
		c.Redirect(http.StatusFound, "/ipam?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/ipam?msg=类别创建成功")
}

func (a *AppContext) ipamSectionDetail(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效类别ID")
		return
	}

	var section models.IPAMSection
	if err := a.DB.First(&section, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=类别不存在")
		return
	}

	subnets, err := a.loadIPAMSubnetsBySection([]uint{section.ID})
	if err != nil {
		renderIPAMError(c, "IPAM 类别", "/ipam", err)
		return
	}
	rootsMap, err := a.loadIPAMCategoryRoots([]uint{section.ID})
	if err != nil {
		renderIPAMError(c, "IPAM 类别", "/ipam", err)
		return
	}
	rootItems := buildCategoryRootItems(rootsMap[section.ID])

	addressCounts, err := a.loadIPAMUsedCountsBySubnet(subnets)
	if err != nil {
		renderIPAMError(c, "IPAM 类别", "/ipam", err)
		return
	}
	items := buildSubnetTreeItems(subnets, addressCounts)

	maxFree := buildSectionMaxFree(rootsMap[section.ID], subnets)

	c.HTML(http.StatusOK, "ipam/section_detail.html", gin.H{
		"Title":     "IPAM 类别详情",
		"Section":   section,
		"Items":     items,
		"Roots":     rootItems,
		"CanManage": currentUser.IsAdmin,
		"MaxFree":   maxFree,
		"Msg":       strings.TrimSpace(c.Query("msg")),
		"Error":     strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) ipamSectionImportPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可批量导入子网")
		return
	}
	sectionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || sectionID == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效类别ID")
		return
	}
	var section models.IPAMSection
	if err := a.DB.First(&section, uint(sectionID)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=类别不存在")
		return
	}
	c.HTML(http.StatusOK, "ipam/section_import.html", gin.H{
		"Title":   "批量导入子网",
		"Section": section,
	})
}

func (a *AppContext) ipamSectionImportTemplate(c *gin.Context) {
	sectionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || sectionID == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效类别ID")
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=ipam-section-%d-template.csv", sectionID))
	writeUTF8BOM(c.Writer)
	writer := newCSVWriter(c.Writer)
	_ = writer.Write([]string{"root", "subnet", "unit", "rate_mbps", "vlan_id", "l2_port", "device", "description"})
	_ = writer.Write([]string{"10.0.0.0/8", "10.0.0.0/29", "示例单位", "100", "20", "Gi0/1", "SW-01", "示例备注"})
	_ = writer.Write([]string{"10.0.0.0/8", "10.0.0.8/30", "", "", "", "", "", "未分配"})
	writer.Flush()
}

func (a *AppContext) ipamSectionImport(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可批量导入子网")
		return
	}
	sectionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || sectionID == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效类别ID")
		return
	}
	var section models.IPAMSection
	if err := a.DB.First(&section, uint(sectionID)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=类别不存在")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/section_import.html", gin.H{
			"Title":   "批量导入子网",
			"Section": section,
			"Error":   "请选择CSV文件",
		})
		return
	}
	reader, err := file.Open()
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/section_import.html", gin.H{
			"Title":   "批量导入子网",
			"Section": section,
			"Error":   "读取文件失败",
		})
		return
	}
	defer reader.Close()

	rows, err := parseIPAMSubnetCSV(reader)
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/section_import.html", gin.H{
			"Title":   "批量导入子网",
			"Section": section,
			"Error":   "CSV解析失败：" + err.Error(),
		})
		return
	}
	validateOnly := c.PostForm("validate_only") == "on"
	if validateOnly {
		result := buildSubnetImportResult(rows)
		c.HTML(http.StatusOK, "ipam/section_import.html", gin.H{
			"Title":   "批量导入子网",
			"Section": section,
			"Result":  result,
		})
		return
	}

	result := applySubnetImportRows(a, section, rows)
	msg := fmt.Sprintf("导入完成：新增%d，更新%d，覆盖删除%d", result.Created, result.Updated, result.Deleted)
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?msg=%s", section.ID, msg))
}

func (a *AppContext) ipamSectionRootAdd(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可管理根网段")
		return
	}

	sectionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || sectionID == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效类别ID")
		return
	}

	var section models.IPAMSection
	if err := a.DB.First(&section, uint(sectionID)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=类别不存在")
		return
	}

	cidr := strings.TrimSpace(c.PostForm("cidr"))
	note := strings.TrimSpace(c.PostForm("note"))
	if cidr == "" {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=根网段不能为空", section.ID))
		return
	}
	prefix, err := parseIPAMPrefix(cidr)
	if err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=%s", section.ID, err.Error()))
		return
	}

	roots, err := a.loadIPAMCategoryRoots([]uint{section.ID})
	if err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=读取根网段失败", section.ID))
		return
	}
	for _, root := range roots[section.ID] {
		existing, err := netip.ParsePrefix(root.CIDR)
		if err != nil {
			continue
		}
		if existing.Addr().Is4() != prefix.Addr().Is4() {
			continue
		}
		if prefixesOverlap(existing, prefix) {
			c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=根网段与现有范围重叠", section.ID))
			return
		}
	}

	var dupCount int64
	if err := a.DB.Model(&models.IPAMCategoryRoot{}).Where("category_id = ? AND cidr = ?", section.ID, prefix.String()).Count(&dupCount).Error; err == nil && dupCount > 0 {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=根网段已存在", section.ID))
		return
	}

	record := models.IPAMCategoryRoot{
		CategoryID: section.ID,
		CIDR:       prefix.String(),
		Note:       note,
	}
	if err := a.DB.Create(&record).Error; err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=新增失败：%s", section.ID, err.Error()))
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?msg=根网段已添加", section.ID))
}

func (a *AppContext) ipamSectionRootDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可管理根网段")
		return
	}

	sectionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || sectionID == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效类别ID")
		return
	}
	rootID, err := strconv.ParseUint(c.Param("rootID"), 10, 64)
	if err != nil || rootID == 0 {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=无效根网段ID", sectionID))
		return
	}

	var root models.IPAMCategoryRoot
	if err := a.DB.First(&root, uint(rootID)).Error; err != nil || root.CategoryID != uint(sectionID) {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=根网段不存在", sectionID))
		return
	}
	prefix, err := netip.ParsePrefix(root.CIDR)
	if err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=根网段无效", sectionID))
		return
	}

	var subnets []models.IPAMSubnet
	if err := a.DB.Where("section_id = ?", sectionID).Find(&subnets).Error; err == nil {
		for _, subnet := range subnets {
			parsed, err := netip.ParsePrefix(subnet.Network)
			if err != nil {
				continue
			}
			if prefixContains(prefix, parsed) {
				c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=根网段内仍有子网，无法删除", sectionID))
				return
			}
		}
	}

	if err := a.DB.Delete(&models.IPAMCategoryRoot{}, uint(rootID)).Error; err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=删除失败：%s", sectionID, err.Error()))
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?msg=根网段已删除", sectionID))
}

func (a *AppContext) ipamSectionEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可编辑类别")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效类别ID")
		return
	}

	var section models.IPAMSection
	if err := a.DB.First(&section, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=类别不存在")
		return
	}

	form := ipamSectionForm{
		ID:   section.ID,
		Name: section.Name,
	}
	c.HTML(http.StatusOK, "ipam/section_form.html", gin.H{
		"Title":  "编辑 IPAM 类别",
		"Action": fmt.Sprintf("/ipam/sections/%d/edit", section.ID),
		"Form":   form,
	})
}

func (a *AppContext) ipamSectionUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可编辑类别")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效类别ID")
		return
	}

	var section models.IPAMSection
	if err := a.DB.First(&section, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=类别不存在")
		return
	}

	form := ipamSectionForm{
		ID:       section.ID,
		Name:     strings.TrimSpace(c.PostForm("name")),
		RootCIDR: strings.TrimSpace(c.PostForm("root_cidr")),
	}
	if form.Name == "" {
		c.HTML(http.StatusBadRequest, "ipam/section_form.html", gin.H{
			"Title":  "编辑 IPAM 类别",
			"Action": fmt.Sprintf("/ipam/sections/%d/edit", section.ID),
			"Form":   form,
			"Error":  "类别名称不能为空",
		})
		return
	}
	section.Name = form.Name
	section.UpdatedAt = time.Now()
	if err := a.DB.Save(&section).Error; err != nil {
		c.HTML(http.StatusBadRequest, "ipam/section_form.html", gin.H{
			"Title":  "编辑 IPAM 类别",
			"Action": fmt.Sprintf("/ipam/sections/%d/edit", section.ID),
			"Form":   form,
			"Error":  "更新失败：" + err.Error(),
		})
		return
	}
	if err := a.ensureSectionRootFromForm(section.ID, form.RootCIDR); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?error=%s", section.ID, err.Error()))
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?msg=更新成功", section.ID))
}

func (a *AppContext) ipamSectionDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可删除类别")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效类别ID")
		return
	}

	if err := a.DB.Where("category_id = ?", uint(id)).Delete(&models.IPAMCategoryRoot{}).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error="+err.Error())
		return
	}
	if err := a.DB.Delete(&models.IPAMSection{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/ipam?msg=类别已删除")
}

func (a *AppContext) ipamSubnetCreatePage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可创建子网")
		return
	}

	sections, _ := a.loadIPAMSections()
	parentID := strings.TrimSpace(c.Query("parent"))
	sectionID := strings.TrimSpace(c.Query("section"))

	c.HTML(http.StatusOK, "ipam/subnet_form.html", gin.H{
		"Title":    "新建子网",
		"Action":   "/ipam/subnets/create",
		"Form":     ipamSubnetForm{ParentID: parentID, SectionID: parseUint(sectionID)},
		"Sections": sections,
		"Subnets":  []models.IPAMSubnet{},
	})
}

func (a *AppContext) ipamSubnetCreate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可创建子网")
		return
	}

	form := readSubnetForm(c)
	sections, _ := a.loadIPAMSections()
	subnets, _ := a.loadIPAMSubnetsBySection([]uint{form.SectionID})

	record, err := bindSubnetForm(form)
	if err != nil {
		a.renderSubnetForm(c, "新建子网", "/ipam/subnets/create", form, sections, subnets, err.Error())
		return
	}
	if err := a.validateSubnetParent(record); err != nil {
		a.renderSubnetForm(c, "新建子网", "/ipam/subnets/create", form, sections, subnets, err.Error())
		return
	}

	if overlap, err := a.ipamHasOverlap(record.Network, 0, record.SectionID, record.ParentID); err != nil {
		a.renderSubnetForm(c, "新建子网", "/ipam/subnets/create", form, sections, subnets, "检查重叠失败："+err.Error())
		return
	} else if overlap {
		a.renderSubnetForm(c, "新建子网", "/ipam/subnets/create", form, sections, subnets, "地址段与已有记录重叠，请检查 CIDR")
		return
	}

	if err := a.DB.Create(&record).Error; err != nil {
		a.renderSubnetForm(c, "新建子网", "/ipam/subnets/create", form, sections, subnets, ipamWriteErrorMessage("创建失败：", err))
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?msg=子网创建成功", record.SectionID))
}

func (a *AppContext) ipamSubnetDetail(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var record models.IPAMSubnet
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}

	recordPrefix, err := netip.ParsePrefix(record.Network)
	if err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网CIDR无效")
		return
	}

	rootPrefix := recordPrefix
	rootLabel := record.Network
	if rootsMap, err := a.loadIPAMCategoryRoots([]uint{record.SectionID}); err == nil {
		rootList := buildRootCIDRList(rootsMap[record.SectionID], "")
		for _, root := range rootList {
			if parsed, err := netip.ParsePrefix(root); err == nil {
				if prefixContains(parsed, recordPrefix) {
					rootPrefix = parsed
					rootLabel = parsed.String()
					break
				}
			}
		}
	}

	var siblings []models.IPAMSubnet
	if err := a.DB.Where("section_id = ?", record.SectionID).Find(&siblings).Error; err != nil {
		renderIPAMError(c, "IPAM 子网", "/ipam", err)
		return
	}

	scopeSubnets := make([]models.IPAMSubnet, 0, len(siblings))
	for _, subnet := range siblings {
		prefix, err := netip.ParsePrefix(subnet.Network)
		if err != nil {
			continue
		}
		if prefixContains(rootPrefix, prefix) {
			scopeSubnets = append(scopeSubnets, subnet)
		}
	}

	usedCount := a.countUsedAddressesBySubnets(scopeSubnets)
	util := buildSubnetUtil(rootPrefix.String(), usedCount)
	freeBlocks := []string{}
	freeBlocks = freeBlocksForSubnet(recordPrefix, siblings, 12)
	blockMap := ipamBlockMap{Note: "仅支持 IPv4 且规模不超过 4096 的子网展示"}
	if recordPrefix.Addr().Is4() && ipv4Total(recordPrefix) <= 4096 {
		var addresses []models.IPAMAddress
		if err := a.DB.Where("subnet_id = ?", record.ID).Find(&addresses).Error; err == nil {
			page := parsePage(c.Query("block_page"))
			blockMap = buildBlockMap(recordPrefix, addresses, page)
		}
	}

	mapSpace := ipamMapSpace{Enabled: false, Note: "IPv6 暂不支持地图空间"}
	if rootPrefix.Addr().Is4() {
		mapSpace = buildMapSpace(rootPrefix, siblings, util, 30, true, nil)
	}

	c.HTML(http.StatusOK, "ipam/subnet_detail.html", gin.H{
		"Title":       "子网详情",
		"Record":      record,
		"Util":        util,
		"FreeBlocks":  freeBlocks,
		"BlockMap":    blockMap,
		"MapSpace":    mapSpace,
		"RootNetwork": rootLabel,
		"CanManage":   currentUser.IsAdmin,
		"IsVirtual":   false,
	})
}

func (a *AppContext) ipamSubnetVirtualDetail(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	sectionID := parseUint(c.Query("section"))
	cidr := strings.TrimSpace(c.Query("cidr"))
	if sectionID == 0 || cidr == "" {
		c.Redirect(http.StatusFound, "/ipam?error=虚拟子网参数缺失")
		return
	}
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() != 24 {
		c.Redirect(http.StatusFound, "/ipam?error=仅支持 IPv4 /24 虚拟子网查看")
		return
	}

	var section models.IPAMSection
	if err := a.DB.First(&section, sectionID).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=类别不存在")
		return
	}

	subnets, _ := a.loadIPAMSubnetsBySection([]uint{sectionID})
	scope := make([]models.IPAMSubnet, 0)
	for _, subnet := range subnets {
		subnetPrefix, err := netip.ParsePrefix(subnet.Network)
		if err != nil {
			continue
		}
		if prefixContains(prefix, subnetPrefix) {
			scope = append(scope, subnet)
		}
	}
	usedCount := a.countUsedAddressesBySubnets(scope)
	util := buildSubnetUtil(prefix.String(), usedCount)
	mapSpace := buildMapSpace(prefix, scope, util, 30, true, nil)
	freeBlocks := freeBlocksForSubnet(prefix, scope, 12)
	blockMap := ipamBlockMap{Note: "虚拟子网不支持地址块视图"}

	virtual := models.IPAMSubnet{
		SectionID: section.ID,
		Network:   prefix.String(),
	}

	c.HTML(http.StatusOK, "ipam/subnet_detail.html", gin.H{
		"Title":       "子网详情",
		"Record":      virtual,
		"Util":        util,
		"FreeBlocks":  freeBlocks,
		"BlockMap":    blockMap,
		"MapSpace":    mapSpace,
		"RootNetwork": prefix.String(),
		"CanManage":   currentUser.IsAdmin,
		"IsVirtual":   true,
	})
}

func (a *AppContext) ipamSubnetEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可编辑子网")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var record models.IPAMSubnet
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}

	sections, _ := a.loadIPAMSections()
	subnets, _ := a.loadIPAMSubnetsBySection([]uint{record.SectionID})

	form := ipamSubnetForm{
		ID:           record.ID,
		SectionID:    record.SectionID,
		ParentID:     uintPtrToString(record.ParentID),
		Network:      record.Network,
		Unit:         record.Unit,
		RateMbps:     strconv.Itoa(record.RateMbps),
		VlanID:       strconv.Itoa(record.VlanID),
		L2Port:       record.L2Port,
		EgressDevice: record.EgressDevice,
		Description:  record.Description,
	}
	c.HTML(http.StatusOK, "ipam/subnet_form.html", gin.H{
		"Title":    "编辑子网",
		"Action":   fmt.Sprintf("/ipam/subnets/%d/edit", record.ID),
		"Form":     form,
		"Sections": sections,
		"Subnets":  subnets,
	})
}

func (a *AppContext) ipamSubnetUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可编辑子网")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var record models.IPAMSubnet
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}

	form := readSubnetForm(c)
	form.ID = record.ID
	sections, _ := a.loadIPAMSections()
	subnets, _ := a.loadIPAMSubnetsBySection([]uint{form.SectionID})

	updated, err := bindSubnetForm(form)
	if err != nil {
		a.renderSubnetForm(c, "编辑子网", fmt.Sprintf("/ipam/subnets/%d/edit", record.ID), form, sections, subnets, err.Error())
		return
	}
	if err := a.validateSubnetParent(updated); err != nil {
		a.renderSubnetForm(c, "编辑子网", fmt.Sprintf("/ipam/subnets/%d/edit", record.ID), form, sections, subnets, err.Error())
		return
	}

	if overlap, err := a.ipamHasOverlap(updated.Network, record.ID, updated.SectionID, updated.ParentID); err != nil {
		a.renderSubnetForm(c, "编辑子网", fmt.Sprintf("/ipam/subnets/%d/edit", record.ID), form, sections, subnets, "检查重叠失败："+err.Error())
		return
	} else if overlap {
		a.renderSubnetForm(c, "编辑子网", fmt.Sprintf("/ipam/subnets/%d/edit", record.ID), form, sections, subnets, "地址段与已有记录重叠，请检查 CIDR")
		return
	}

	record.SectionID = updated.SectionID
	record.ParentID = updated.ParentID
	record.Network = updated.Network
	record.Unit = updated.Unit
	record.RateMbps = updated.RateMbps
	record.VlanID = updated.VlanID
	record.L2Port = updated.L2Port
	record.EgressDevice = updated.EgressDevice
	record.Description = updated.Description
	record.UpdatedAt = time.Now()

	if err := a.DB.Save(&record).Error; err != nil {
		a.renderSubnetForm(c, "编辑子网", fmt.Sprintf("/ipam/subnets/%d/edit", record.ID), form, sections, subnets, ipamWriteErrorMessage("更新失败：", err))
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d?msg=更新成功", record.ID))
}

func (a *AppContext) ipamSubnetDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可删除子网")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var subnet models.IPAMSubnet
	if err := a.DB.First(&subnet, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}

	if err := a.DB.Delete(&models.IPAMSubnet{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?msg=子网已删除", subnet.SectionID))
}

func (a *AppContext) ipamSubnetSplitPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可划分子网")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var subnet models.IPAMSubnet
	if err := a.DB.First(&subnet, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}
	prefix, err := netip.ParsePrefix(subnet.Network)
	if err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网CIDR无效")
		return
	}
	defaultBits := prefix.Bits() + 1
	form := ipamSplitForm{
		SubnetID:  subnet.ID,
		Network:   subnet.Network,
		NewPrefix: strconv.Itoa(defaultBits),
	}
	c.HTML(http.StatusOK, "ipam/subnet_split.html", gin.H{
		"Title":  "子网自动划分",
		"Subnet": subnet,
		"Form":   form,
	})
}

func (a *AppContext) ipamSubnetSplit(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可划分子网")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var subnet models.IPAMSubnet
	if err := a.DB.First(&subnet, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}
	prefix, err := netip.ParsePrefix(subnet.Network)
	if err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网CIDR无效")
		return
	}

	form := ipamSplitForm{
		SubnetID:  subnet.ID,
		Network:   subnet.Network,
		NewPrefix: strings.TrimSpace(c.PostForm("new_prefix")),
	}
	newBits, err := strconv.Atoi(form.NewPrefix)
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/subnet_split.html", gin.H{
			"Title":  "子网自动划分",
			"Subnet": subnet,
			"Form":   form,
			"Error":  "子网掩码格式错误",
		})
		return
	}
	addrBits := 32
	if prefix.Addr().Is6() {
		addrBits = 128
	}
	if newBits <= prefix.Bits() || newBits > addrBits {
		c.HTML(http.StatusBadRequest, "ipam/subnet_split.html", gin.H{
			"Title":  "子网自动划分",
			"Subnet": subnet,
			"Form":   form,
			"Error":  fmt.Sprintf("掩码必须在 /%d 到 /%d 之间", prefix.Bits()+1, addrBits),
		})
		return
	}

	maxCount := 4096
	if addrBits == 128 {
		maxCount = 256
	}
	delta := newBits - prefix.Bits()
	if delta > 12 && addrBits == 32 {
		c.HTML(http.StatusBadRequest, "ipam/subnet_split.html", gin.H{
			"Title":  "子网自动划分",
			"Subnet": subnet,
			"Form":   form,
			"Error":  "划分数量过多，请选择更大的掩码",
		})
		return
	}
	count := 1 << delta
	if count > maxCount {
		c.HTML(http.StatusBadRequest, "ipam/subnet_split.html", gin.H{
			"Title":  "子网自动划分",
			"Subnet": subnet,
			"Form":   form,
			"Error":  "划分数量过多，请选择更大的掩码",
		})
		return
	}
	form.Count = count

	var existing []models.IPAMSubnet
	if err := a.DB.Where("parent_id = ? AND section_id = ?", subnet.ID, subnet.SectionID).Find(&existing).Error; err != nil {
		c.HTML(http.StatusBadRequest, "ipam/subnet_split.html", gin.H{
			"Title":  "子网自动划分",
			"Subnet": subnet,
			"Form":   form,
			"Error":  "读取已有子网失败：" + err.Error(),
		})
		return
	}
	if len(existing) > 0 {
		c.HTML(http.StatusBadRequest, "ipam/subnet_split.html", gin.H{
			"Title":  "子网自动划分",
			"Subnet": subnet,
			"Form":   form,
			"Error":  "该子网已存在子网，请先清理后再自动划分",
		})
		return
	}

	children := splitPrefixList(prefix, newBits, count)
	tx := a.DB.Begin()
	for _, child := range children {
		record := models.IPAMSubnet{
			SectionID:    subnet.SectionID,
			ParentID:     &subnet.ID,
			Network:      child.String(),
			Unit:         subnet.Unit,
			VRF:          subnet.VRF,
			RateMbps:     subnet.RateMbps,
			VlanID:       subnet.VlanID,
			L2Port:       subnet.L2Port,
			EgressDevice: subnet.EgressDevice,
			Description:  fmt.Sprintf("来自 %s 自动划分", subnet.Network),
		}
		if err := tx.Create(&record).Error; err != nil {
			tx.Rollback()
			c.HTML(http.StatusBadRequest, "ipam/subnet_split.html", gin.H{
				"Title":  "子网自动划分",
				"Subnet": subnet,
				"Form":   form,
				"Error":  "创建子网失败：" + err.Error(),
			})
			return
		}
	}
	tx.Commit()
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/sections/%d?msg=子网已划分", subnet.SectionID))
}

func (a *AppContext) ipamAddressList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var subnet models.IPAMSubnet
	if err := a.DB.First(&subnet, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}

	var addresses []models.IPAMAddress
	if err := a.DB.Where("subnet_id = ?", subnet.ID).Order("ip").Find(&addresses).Error; err != nil {
		renderIPAMError(c, "IP 地址管理", "/ipam", err)
		return
	}

	items := make([]ipamAddressItem, 0, len(addresses))
	for _, addr := range addresses {
		items = append(items, ipamAddressItem{
			ID:        addr.ID,
			IP:        addr.IP,
			Status:    ipamStatusLabel(addr.Status),
			Unit:      addr.Unit,
			Hostname:  addr.Hostname,
			Note:      addr.Note,
			UpdatedAt: addr.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}

	c.HTML(http.StatusOK, "ipam/address_list.html", gin.H{
		"Title":     "IP 地址管理",
		"Subnet":    subnet,
		"Items":     items,
		"CanManage": currentUser.IsAdmin,
		"Msg":       strings.TrimSpace(c.Query("msg")),
		"Error":     strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) ipamAddressGeneratePage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可生成地址池")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var subnet models.IPAMSubnet
	if err := a.DB.First(&subnet, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}

	form := ipamGenerateForm{SubnetID: subnet.ID, Status: "used"}
	c.HTML(http.StatusOK, "ipam/address_generate.html", gin.H{
		"Title":   "地址池批量生成",
		"Subnet":  subnet,
		"Form":    form,
		"Options": ipamStatusOptions,
	})
}

func (a *AppContext) ipamAddressGenerate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可生成地址池")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var subnet models.IPAMSubnet
	if err := a.DB.First(&subnet, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}

	form := ipamGenerateForm{
		SubnetID:       subnet.ID,
		StartIP:        strings.TrimSpace(c.PostForm("start_ip")),
		EndIP:          strings.TrimSpace(c.PostForm("end_ip")),
		Count:          strings.TrimSpace(c.PostForm("count")),
		Status:         normalizeIPAMStatus(c.PostForm("status")),
		Unit:           strings.TrimSpace(c.PostForm("unit")),
		HostnamePrefix: strings.TrimSpace(c.PostForm("hostname_prefix")),
		Note:           strings.TrimSpace(c.PostForm("note")),
	}

	result, err := a.generateAddressPool(subnet, form, c.PostForm("validate_only") == "on")
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/address_generate.html", gin.H{
			"Title":   "地址池批量生成",
			"Subnet":  subnet,
			"Form":    form,
			"Options": ipamStatusOptions,
			"Error":   err.Error(),
			"Result":  result,
		})
		return
	}

	if c.PostForm("validate_only") == "on" {
		c.HTML(http.StatusOK, "ipam/address_generate.html", gin.H{
			"Title":   "地址池批量生成",
			"Subnet":  subnet,
			"Form":    form,
			"Options": ipamStatusOptions,
			"Result":  result,
		})
		return
	}

	msg := fmt.Sprintf("生成完成：新增%d，跳过%d", result.Created, result.Skipped)
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?msg=%s", subnet.ID, msg))
}

func (a *AppContext) ipamAddressBulkUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可批量操作")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	ids := c.PostFormArray("ids")
	if len(ids) == 0 {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?error=请选择地址", id))
		return
	}
	action := strings.TrimSpace(c.PostForm("action"))
	if action == "" {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?error=请选择批量操作", id))
		return
	}

	idList := make([]uint, 0, len(ids))
	for _, raw := range ids {
		val, err := strconv.ParseUint(raw, 10, 64)
		if err == nil && val > 0 {
			idList = append(idList, uint(val))
		}
	}
	if len(idList) == 0 {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?error=地址选择无效", id))
		return
	}

	if action == "delete" {
		if err := a.DB.Where("subnet_id = ? AND id IN ?", id, idList).Delete(&models.IPAMAddress{}).Error; err != nil {
			c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?error=删除失败", id))
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?msg=已删除所选地址", id))
		return
	}

	status := normalizeIPAMStatus(action)
	if err := a.DB.Model(&models.IPAMAddress{}).Where("subnet_id = ? AND id IN ?", id, idList).Updates(map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}).Error; err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?error=批量更新失败", id))
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?msg=已更新所选地址状态", id))
}

func (a *AppContext) ipamAddressImportPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可导入地址")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var subnet models.IPAMSubnet
	if err := a.DB.First(&subnet, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}

	c.HTML(http.StatusOK, "ipam/address_import.html", gin.H{
		"Title":  "批量导入地址",
		"Subnet": subnet,
	})
}

func (a *AppContext) ipamAddressImport(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可导入地址")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var subnet models.IPAMSubnet
	if err := a.DB.First(&subnet, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/address_import.html", gin.H{
			"Title":  "批量导入地址",
			"Subnet": subnet,
			"Error":  "请选择CSV文件",
		})
		return
	}
	reader, err := file.Open()
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/address_import.html", gin.H{
			"Title":  "批量导入地址",
			"Subnet": subnet,
			"Error":  "读取文件失败",
		})
		return
	}
	defer reader.Close()

	rows, err := parseIPAMCSV(reader)
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/address_import.html", gin.H{
			"Title":  "批量导入地址",
			"Subnet": subnet,
			"Error":  "CSV解析失败：" + err.Error(),
		})
		return
	}

	validateOnly := c.PostForm("validate_only") == "on"
	if validateOnly {
		result := buildImportResult(rows)
		c.HTML(http.StatusOK, "ipam/address_import.html", gin.H{
			"Title":  "批量导入地址",
			"Subnet": subnet,
			"Result": result,
		})
		return
	}

	result := applyImportRows(a, subnet, rows)
	msg := fmt.Sprintf("导入完成：新增%d，更新%d，跳过%d", result.Created, result.Updated, result.Skipped)
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?msg=%s", subnet.ID, msg))
}

func (a *AppContext) ipamAddressExport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	var subnet models.IPAMSubnet
	if err := a.DB.First(&subnet, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=子网不存在")
		return
	}

	var addresses []models.IPAMAddress
	if err := a.DB.Where("subnet_id = ?", subnet.ID).Order("ip").Find(&addresses).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=导出失败")
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=ipam-subnet-%d.csv", subnet.ID))

	writeUTF8BOM(c.Writer)
	writer := newCSVWriter(c.Writer)
	_ = writer.Write([]string{"ip", "status", "unit", "hostname", "note"})
	for _, addr := range addresses {
		_ = writer.Write([]string{addr.IP, addr.Status, addr.Unit, addr.Hostname, addr.Note})
	}
	writer.Flush()
}

func (a *AppContext) ipamAddressTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=ipam-template-%d.csv", id))
	writeUTF8BOM(c.Writer)
	writer := newCSVWriter(c.Writer)
	_ = writer.Write([]string{"ip", "status", "unit", "hostname", "note"})
	_ = writer.Write([]string{"10.0.0.10", "used", "示例单位", "host-01", "示例备注"})
	writer.Flush()
}

func (a *AppContext) ipamAddressCreatePage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可创建地址")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	form := ipamAddressForm{SubnetID: uint(id), Status: "used"}
	c.HTML(http.StatusOK, "ipam/address_form.html", gin.H{
		"Title":   "新增 IP 地址",
		"Action":  fmt.Sprintf("/ipam/subnets/%d/addresses/create", id),
		"Form":    form,
		"Options": ipamStatusOptions,
	})
}

func (a *AppContext) ipamAddressCreate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可创建地址")
		return
	}

	subnetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || subnetID == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效子网ID")
		return
	}

	form := readAddressForm(c)
	form.SubnetID = uint(subnetID)

	addr, err := bindAddressForm(form)
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/address_form.html", gin.H{
			"Title":   "新增 IP 地址",
			"Action":  fmt.Sprintf("/ipam/subnets/%d/addresses/create", subnetID),
			"Form":    form,
			"Options": ipamStatusOptions,
			"Error":   err.Error(),
		})
		return
	}

	if err := a.DB.Create(&addr).Error; err != nil {
		c.HTML(http.StatusBadRequest, "ipam/address_form.html", gin.H{
			"Title":   "新增 IP 地址",
			"Action":  fmt.Sprintf("/ipam/subnets/%d/addresses/create", subnetID),
			"Form":    form,
			"Options": ipamStatusOptions,
			"Error":   "创建失败：" + err.Error(),
		})
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?msg=地址已创建", subnetID))
}

func (a *AppContext) ipamAddressEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可编辑地址")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效地址ID")
		return
	}

	var addr models.IPAMAddress
	if err := a.DB.First(&addr, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=地址不存在")
		return
	}

	form := ipamAddressForm{
		ID:       addr.ID,
		SubnetID: addr.SubnetID,
		IP:       addr.IP,
		Status:   addr.Status,
		Unit:     addr.Unit,
		Hostname: addr.Hostname,
		Note:     addr.Note,
	}
	c.HTML(http.StatusOK, "ipam/address_form.html", gin.H{
		"Title":   "编辑 IP 地址",
		"Action":  fmt.Sprintf("/ipam/addresses/%d/edit", addr.ID),
		"Form":    form,
		"Options": ipamStatusOptions,
	})
}

func (a *AppContext) ipamAddressUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可编辑地址")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效地址ID")
		return
	}

	var addr models.IPAMAddress
	if err := a.DB.First(&addr, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=地址不存在")
		return
	}

	form := readAddressForm(c)
	form.ID = addr.ID
	form.SubnetID = addr.SubnetID

	updated, err := bindAddressForm(form)
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/address_form.html", gin.H{
			"Title":   "编辑 IP 地址",
			"Action":  fmt.Sprintf("/ipam/addresses/%d/edit", addr.ID),
			"Form":    form,
			"Options": ipamStatusOptions,
			"Error":   err.Error(),
		})
		return
	}

	addr.IP = updated.IP
	addr.Status = updated.Status
	addr.Unit = updated.Unit
	addr.Hostname = updated.Hostname
	addr.Note = updated.Note
	addr.UpdatedAt = time.Now()

	if err := a.DB.Save(&addr).Error; err != nil {
		c.HTML(http.StatusBadRequest, "ipam/address_form.html", gin.H{
			"Title":   "编辑 IP 地址",
			"Action":  fmt.Sprintf("/ipam/addresses/%d/edit", addr.ID),
			"Form":    form,
			"Options": ipamStatusOptions,
			"Error":   "更新失败：" + err.Error(),
		})
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?msg=地址已更新", addr.SubnetID))
}

func (a *AppContext) ipamAddressDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可删除地址")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效地址ID")
		return
	}

	var addr models.IPAMAddress
	if err := a.DB.First(&addr, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=地址不存在")
		return
	}

	if err := a.DB.Delete(&models.IPAMAddress{}, addr.ID).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/ipam/subnets/%d/addresses?msg=地址已删除", addr.SubnetID))
}

func (a *AppContext) renderSubnetForm(c *gin.Context, title, action string, form ipamSubnetForm, sections []models.IPAMSection, subnets []models.IPAMSubnet, errMsg string) {
	c.HTML(http.StatusBadRequest, "ipam/subnet_form.html", gin.H{
		"Title":    title,
		"Action":   action,
		"Form":     form,
		"Sections": sections,
		"Subnets":  subnets,
		"Error":    errMsg,
	})
}

func (a *AppContext) ipamHasOverlap(cidr string, excludeID uint, sectionID uint, parentID *uint) (bool, error) {
	query := a.DB.Model(&models.IPAMSubnet{}).Where("network && ?::cidr", cidr).Where("section_id = ?", sectionID)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func readSubnetForm(c *gin.Context) ipamSubnetForm {
	return ipamSubnetForm{
		SectionID:    parseUint(c.PostForm("section_id")),
		ParentID:     strings.TrimSpace(c.PostForm("parent_id")),
		Network:      strings.TrimSpace(c.PostForm("network")),
		Unit:         strings.TrimSpace(c.PostForm("unit")),
		RateMbps:     strings.TrimSpace(c.PostForm("rate_mbps")),
		VlanID:       strings.TrimSpace(c.PostForm("vlan_id")),
		L2Port:       strings.TrimSpace(c.PostForm("l2_port")),
		EgressDevice: strings.TrimSpace(c.PostForm("egress_device")),
		Description:  strings.TrimSpace(c.PostForm("description")),
	}
}

func bindSubnetForm(form ipamSubnetForm) (models.IPAMSubnet, error) {
	if form.SectionID == 0 {
		return models.IPAMSubnet{}, fmt.Errorf("请选择所属类别")
	}
	prefix, err := parseIPAMPrefix(form.Network)
	if err != nil {
		return models.IPAMSubnet{}, err
	}
	if form.Unit == "" {
		return models.IPAMSubnet{}, fmt.Errorf("客户单位名称不能为空")
	}
	if form.L2Port == "" {
		return models.IPAMSubnet{}, fmt.Errorf("二层节点端口不能为空")
	}
	if form.EgressDevice == "" {
		return models.IPAMSubnet{}, fmt.Errorf("三层设备名称不能为空")
	}
	rate, err := parsePositiveInt(form.RateMbps, "速率(Mbps)", 1, 1000000)
	if err != nil {
		return models.IPAMSubnet{}, err
	}
	vlan, err := parsePositiveInt(form.VlanID, "VLAN ID", 1, 4094)
	if err != nil {
		return models.IPAMSubnet{}, err
	}
	var parentID *uint
	if strings.TrimSpace(form.ParentID) != "" {
		pid := parseUint(form.ParentID)
		if pid > 0 {
			parentID = &pid
		}
	}

	record := models.IPAMSubnet{
		SectionID:    form.SectionID,
		ParentID:     parentID,
		Network:      prefix.String(),
		Unit:         form.Unit,
		VRF:          "default",
		RateMbps:     rate,
		VlanID:       vlan,
		L2Port:       form.L2Port,
		EgressDevice: form.EgressDevice,
		Description:  form.Description,
	}
	return record, nil
}

func readAddressForm(c *gin.Context) ipamAddressForm {
	return ipamAddressForm{
		IP:       strings.TrimSpace(c.PostForm("ip")),
		Status:   strings.TrimSpace(c.PostForm("status")),
		Unit:     strings.TrimSpace(c.PostForm("unit")),
		Hostname: strings.TrimSpace(c.PostForm("hostname")),
		Note:     strings.TrimSpace(c.PostForm("note")),
	}
}

func bindAddressForm(form ipamAddressForm) (models.IPAMAddress, error) {
	if form.SubnetID == 0 {
		return models.IPAMAddress{}, fmt.Errorf("子网ID无效")
	}
	if form.IP == "" {
		return models.IPAMAddress{}, fmt.Errorf("IP 地址不能为空")
	}
	if _, err := netip.ParseAddr(form.IP); err != nil {
		return models.IPAMAddress{}, fmt.Errorf("IP 地址格式错误")
	}
	status := form.Status
	if status == "" {
		status = "used"
	}
	return models.IPAMAddress{
		SubnetID: form.SubnetID,
		IP:       form.IP,
		Status:   status,
		Unit:     form.Unit,
		Hostname: form.Hostname,
		Note:     form.Note,
	}, nil
}

func (a *AppContext) loadIPAMSections() ([]models.IPAMSection, error) {
	var sections []models.IPAMSection
	if err := a.DB.Order("id").Find(&sections).Error; err != nil {
		return nil, err
	}
	return sections, nil
}

func (a *AppContext) loadIPAMCategoryRoots(sectionIDs []uint) (map[uint][]models.IPAMCategoryRoot, error) {
	result := make(map[uint][]models.IPAMCategoryRoot)
	if len(sectionIDs) == 0 {
		return result, nil
	}
	var roots []models.IPAMCategoryRoot
	if err := a.DB.Where("category_id IN ?", sectionIDs).Order("cidr").Find(&roots).Error; err != nil {
		return nil, err
	}
	for _, root := range roots {
		result[root.CategoryID] = append(result[root.CategoryID], root)
	}
	return result, nil
}

func (a *AppContext) loadIPAMSubnetsBySection(sectionIDs []uint) ([]models.IPAMSubnet, error) {
	if len(sectionIDs) == 0 {
		return []models.IPAMSubnet{}, nil
	}
	var subnets []models.IPAMSubnet
	if err := a.DB.Where("section_id IN ?", sectionIDs).Order("network").Find(&subnets).Error; err != nil {
		return nil, err
	}
	return subnets, nil
}

func (a *AppContext) loadIPAMUsedCountsBySubnet(subnets []models.IPAMSubnet) (map[uint]int64, error) {
	result := make(map[uint]int64)
	if len(subnets) == 0 {
		return result, nil
	}
	ids := make([]uint, 0, len(subnets))
	for _, s := range subnets {
		ids = append(ids, s.ID)
	}
	rows, err := a.DB.Raw(
		`SELECT subnet_id, COUNT(*) FROM ip_am_addresses WHERE subnet_id IN ? AND unit <> '' GROUP BY subnet_id`,
		ids,
	).Rows()
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var subnetID uint
		var count int64
		if err := rows.Scan(&subnetID, &count); err == nil {
			result[subnetID] = count
		}
	}
	return result, nil
}

func (a *AppContext) buildSectionItems(sections []models.IPAMSection, subnets []models.IPAMSubnet, used map[uint]int64, roots map[uint][]models.IPAMCategoryRoot) []ipamSectionItem {
	items := make([]ipamSectionItem, 0, len(sections))
	bySection := make(map[uint][]models.IPAMSubnet)
	for _, s := range subnets {
		bySection[s.SectionID] = append(bySection[s.SectionID], s)
	}
	for _, section := range sections {
		list := bySection[section.ID]
		rootList := roots[section.ID]
		rootCIDRs := buildRootCIDRList(rootList, section.RootCIDR)
		var v4Used int64
		var v4Total int64
		var v6Used int64
		for _, subnet := range list {
			count := used[subnet.ID]
			prefix, err := netip.ParsePrefix(subnet.Network)
			if err != nil {
				continue
			}
			if prefix.Addr().Is4() {
				v4Used += count
				v4Total += ipv4Total(prefix)
			} else {
				v6Used += count
			}
		}
		percent := 0
		percentText := "-"
		if v4Total > 0 {
			percent = int(math.Round(float64(v4Used) / float64(v4Total) * 100))
			percentText = fmt.Sprintf("%d%%", percent)
		}
		maxFree, maxHint := buildMaxFreeHint(rootList, section.RootCIDR, list)
		items = append(items, ipamSectionItem{
			ID:              section.ID,
			Name:            section.Name,
			RootCIDRs:       rootCIDRs,
			RootCount:       len(rootCIDRs),
			SubnetCount:     len(list),
			IPv4Used:        v4Used,
			IPv4Total:       v4Total,
			IPv6Used:        v6Used,
			MaxFreePrefix:   maxFree,
			MaxFreeHint:     maxHint,
			UtilPercent:     percent,
			UtilPercentText: percentText,
		})
	}
	return items
}

type ipamOverviewRoot struct {
	Key      string
	CIDR     string
	Selected bool
}

type ipamOverviewGroup struct {
	SectionID   uint
	SectionName string
	Roots       []ipamOverviewRoot
}

type ipamOverview struct {
	Enabled         bool
	RootLabel       string
	FreePercent     int
	FreePercentText string
	MapSpace        ipamMapSpace
	Groups          []ipamOverviewGroup
	SelectedKey     string
}

func buildIPAMOverview(sections []models.IPAMSection, subnets []models.IPAMSubnet, used map[uint]int64, roots map[uint][]models.IPAMCategoryRoot, selectedKey string) ipamOverview {
	groups := make([]ipamOverviewGroup, 0, len(sections))
	var selectedSectionID uint
	var selectedRoot string
	var selectedSectionName string
	selectedSet := false

	for _, section := range sections {
		rootList := roots[section.ID]
		rootCIDRs := buildRootCIDRList(rootList, section.RootCIDR)
		if len(rootCIDRs) == 0 {
			continue
		}
		group := ipamOverviewGroup{SectionID: section.ID, SectionName: section.Name}
		for _, root := range rootCIDRs {
			key := fmt.Sprintf("%d|%s", section.ID, root)
			isSelected := selectedKey != "" && key == selectedKey
			if !selectedSet && (isSelected || selectedKey == "") {
				selectedSet = true
				selectedSectionID = section.ID
				selectedRoot = root
				selectedSectionName = section.Name
				selectedKey = key
				isSelected = true
			}
			group.Roots = append(group.Roots, ipamOverviewRoot{
				Key:      key,
				CIDR:     root,
				Selected: isSelected,
			})
		}
		groups = append(groups, group)
	}

	if !selectedSet {
		return ipamOverview{
			Enabled:   false,
			RootLabel: "暂无根网段",
			MapSpace:  ipamMapSpace{Enabled: false, Note: "暂无根网段"},
			Groups:    groups,
		}
	}

	prefix, err := netip.ParsePrefix(selectedRoot)
	if err != nil {
		return ipamOverview{
			Enabled:     false,
			RootLabel:   "根网段格式不正确",
			MapSpace:    ipamMapSpace{Enabled: false, Note: "根网段格式不正确"},
			Groups:      groups,
			SelectedKey: selectedKey,
		}
	}

	scope := make([]models.IPAMSubnet, 0)
	var usedCount int64
	for _, subnet := range subnets {
		if subnet.SectionID != selectedSectionID {
			continue
		}
		subnetPrefix, err := netip.ParsePrefix(subnet.Network)
		if err != nil {
			continue
		}
		if !prefixContains(prefix, subnetPrefix) {
			continue
		}
		scope = append(scope, subnet)
		usedCount += used[subnet.ID]
	}

	util := buildSubnetUtil(selectedRoot, usedCount)
	linkByCIDR := make(map[string]string)
	existing := make(map[string]uint)
	for _, subnet := range scope {
		p, err := netip.ParsePrefix(subnet.Network)
		if err != nil {
			continue
		}
		if p.Bits() != 24 {
			continue
		}
		if prefixContains(prefix, p) {
			existing[p.String()] = subnet.ID
		}
	}
	if prefix.Bits() <= 24 {
		total := 1 << uint(24-prefix.Bits())
		blocks := splitPrefixList(prefix, 24, total)
		for _, block := range blocks {
			cidr := block.String()
			if id, ok := existing[cidr]; ok {
				linkByCIDR[cidr] = fmt.Sprintf("/ipam/subnets/%d", id)
			} else {
				linkByCIDR[cidr] = fmt.Sprintf("/ipam/subnets/virtual?section=%d&cidr=%s", selectedSectionID, url.QueryEscape(cidr))
			}
		}
	}
	maxBits := 24
	if prefix.Bits() >= 24 {
		maxBits = 30
	}
	mapSpace := buildMapSpace(prefix, scope, util, maxBits, false, linkByCIDR)
	freePercent := 0
	freeText := "-"
	if prefix.Addr().Is4() {
		total := ipv4Total(prefix)
		if total > 0 {
			freePercent = int(math.Round(float64(total-usedCount) / float64(total) * 100))
			if freePercent < 0 {
				freePercent = 0
			}
			if freePercent > 100 {
				freePercent = 100
			}
			freeText = fmt.Sprintf("%d%%", freePercent)
		}
	}

	return ipamOverview{
		Enabled:         true,
		RootLabel:       fmt.Sprintf("%s · %s", selectedSectionName, selectedRoot),
		FreePercent:     freePercent,
		FreePercentText: freeText,
		MapSpace:        mapSpace,
		Groups:          groups,
		SelectedKey:     selectedKey,
	}
}

func buildSubnetTreeItems(subnets []models.IPAMSubnet, used map[uint]int64) []ipamSubnetItem {
	byParent := make(map[uint][]models.IPAMSubnet)
	childCount := make(map[uint]int)
	var roots []models.IPAMSubnet
	for _, subnet := range subnets {
		if subnet.ParentID != nil {
			byParent[*subnet.ParentID] = append(byParent[*subnet.ParentID], subnet)
			childCount[*subnet.ParentID]++
		} else {
			roots = append(roots, subnet)
		}
	}
	for key := range byParent {
		sort.Slice(byParent[key], func(i, j int) bool {
			return byParent[key][i].Network < byParent[key][j].Network
		})
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Network < roots[j].Network })

	items := make([]ipamSubnetItem, 0, len(subnets))
	var walk func(list []models.IPAMSubnet, depth int)
	walk = func(list []models.IPAMSubnet, depth int) {
		for _, subnet := range list {
			util := buildSubnetUtil(subnet.Network, used[subnet.ID])
			parentID := uint(0)
			if subnet.ParentID != nil {
				parentID = *subnet.ParentID
			}
			items = append(items, ipamSubnetItem{
				ID:           subnet.ID,
				ParentID:     parentID,
				Network:      subnet.Network,
				Unit:         subnet.Unit,
				RateMbps:     subnet.RateMbps,
				VlanID:       subnet.VlanID,
				L2Port:       subnet.L2Port,
				EgressDevice: subnet.EgressDevice,
				UpdatedAt:    subnet.UpdatedAt.Format("2006-01-02 15:04"),
				UsedCount:    util.Used,
				TotalCount:   util.TotalText,
				UtilPercent:  util.PercentText,
				Depth:        depth,
				IsIPv6:       util.IsIPv6,
				HasChildren:  childCount[subnet.ID] > 0,
			})
			if children, ok := byParent[subnet.ID]; ok {
				walk(children, depth+1)
			}
		}
	}
	walk(roots, 0)
	return items
}

func buildCategoryRootItems(roots []models.IPAMCategoryRoot) []ipamCategoryRootItem {
	items := make([]ipamCategoryRootItem, 0, len(roots))
	for _, root := range roots {
		items = append(items, ipamCategoryRootItem{
			ID:   root.ID,
			CIDR: root.CIDR,
			Note: root.Note,
		})
	}
	return items
}

func buildRootCIDRList(roots []models.IPAMCategoryRoot, legacy string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(roots)+1)
	for _, root := range roots {
		if root.CIDR == "" {
			continue
		}
		if _, ok := seen[root.CIDR]; ok {
			continue
		}
		seen[root.CIDR] = struct{}{}
		result = append(result, root.CIDR)
	}
	if legacy != "" {
		if _, ok := seen[legacy]; !ok {
			result = append(result, legacy)
		}
	}
	sort.Strings(result)
	return result
}

func buildMaxFreeHint(roots []models.IPAMCategoryRoot, legacy string, subnets []models.IPAMSubnet) (string, string) {
	rootList := buildRootCIDRList(roots, legacy)
	if len(rootList) == 0 {
		return "未配置", ""
	}
	best := ""
	bestRoot := ""
	bestBits := 129
	for _, rootCIDR := range rootList {
		prefix, err := parseIPAMPrefix(rootCIDR)
		if err != nil {
			continue
		}
		freePrefix := findLargestFreePrefix(prefix, subnets)
		if freePrefix == "" {
			continue
		}
		parsed, err := netip.ParsePrefix(freePrefix)
		if err != nil {
			continue
		}
		if parsed.Bits() < bestBits {
			bestBits = parsed.Bits()
			best = freePrefix
			bestRoot = prefix.String()
		}
	}
	if best == "" {
		return "无空闲", ""
	}
	return best, bestRoot
}

func buildSectionMaxFree(roots []models.IPAMCategoryRoot, subnets []models.IPAMSubnet) string {
	free, hint := buildMaxFreeHint(roots, "", subnets)
	if hint == "" {
		return free
	}
	return fmt.Sprintf("%s（%s）", free, hint)
}

func (a *AppContext) searchIPAM(searchIP, searchUnit string) ([]ipamSearchSubnet, []ipamSearchAddress) {
	var subnetMatches []ipamSearchSubnet
	var addressMatches []ipamSearchAddress

	if searchIP != "" {
		addr, addrErr := netip.ParseAddr(searchIP)
		if addrErr == nil {
			var addrRows []struct {
				ID           uint
				IP           string
				Status       string
				Unit         string
				Hostname     string
				Note         string
				SubnetID     uint
				Network      string
				Section      string
				RateMbps     int
				VlanID       int
				L2Port       string
				EgressDevice string
				UpdatedAt    time.Time
			}
			a.DB.Table("ip_am_addresses").
				Select("ip_am_addresses.id, ip_am_addresses.ip, ip_am_addresses.status, ip_am_addresses.unit, ip_am_addresses.hostname, ip_am_addresses.note, ip_am_addresses.subnet_id, ip_am_subnets.network, ip_am_sections.name as section, ip_am_subnets.rate_mbps, ip_am_subnets.vlan_id, ip_am_subnets.l2_port, ip_am_subnets.egress_device, ip_am_addresses.updated_at").
				Joins("left join ip_am_subnets on ip_am_subnets.id = ip_am_addresses.subnet_id").
				Joins("left join ip_am_sections on ip_am_sections.id = ip_am_subnets.section_id").
				Where("ip_am_addresses.ip = ?", addr.String()).Find(&addrRows)
			for _, row := range addrRows {
				statusLabel := ipamStatusLabel(row.Status)
				if strings.TrimSpace(row.Unit) != "" {
					statusLabel = "已使用"
				}
				addressMatches = append(addressMatches, ipamSearchAddress{
					ID:           row.ID,
					IP:           row.IP,
					Status:       statusLabel,
					Unit:         row.Unit,
					Hostname:     row.Hostname,
					Note:         row.Note,
					SubnetID:     row.SubnetID,
					Network:      row.Network,
					Section:      row.Section,
					RateMbps:     row.RateMbps,
					VlanID:       row.VlanID,
					L2Port:       row.L2Port,
					EgressDevice: row.EgressDevice,
					UpdatedAt:    row.UpdatedAt.Format("2006-01-02 15:04"),
				})
			}

			var subRows []struct {
				ID           uint
				Network      string
				Unit         string
				Section      string
				RateMbps     int
				VlanID       int
				L2Port       string
				EgressDevice string
				Description  string
				UpdatedAt    time.Time
			}
			a.DB.Table("ip_am_subnets").
				Select("ip_am_subnets.id, ip_am_subnets.network, ip_am_subnets.unit, ip_am_subnets.rate_mbps, ip_am_subnets.vlan_id, ip_am_subnets.l2_port, ip_am_subnets.egress_device, ip_am_subnets.description, ip_am_subnets.updated_at, ip_am_sections.name as section").
				Joins("left join ip_am_sections on ip_am_sections.id = ip_am_subnets.section_id").
				Where("?::inet <<= ip_am_subnets.network", addr.String()).Order("masklen(ip_am_subnets.network) desc").Find(&subRows)
			subnetList := make([]models.IPAMSubnet, 0, len(subRows))
			for _, row := range subRows {
				subnetList = append(subnetList, models.IPAMSubnet{ID: row.ID, Network: row.Network})
			}
			usedMap, _ := a.loadIPAMUsedCountsBySubnet(subnetList)
			for _, row := range subRows {
				util := buildSubnetUtil(row.Network, usedMap[row.ID])
				subnetMatches = append(subnetMatches, ipamSearchSubnet{
					ID:           row.ID,
					Network:      row.Network,
					Unit:         row.Unit,
					Section:      row.Section,
					RateMbps:     row.RateMbps,
					VlanID:       row.VlanID,
					L2Port:       row.L2Port,
					EgressDevice: row.EgressDevice,
					Description:  row.Description,
					UpdatedAt:    row.UpdatedAt.Format("2006-01-02 15:04"),
					UsedCount:    util.Used,
					TotalCount:   util.TotalText,
					UtilPercent:  util.PercentText,
				})
			}
		}
	}

	if searchUnit != "" {
		like := "%" + searchUnit + "%"
		var subRows []struct {
			ID           uint
			Network      string
			Unit         string
			Section      string
			RateMbps     int
			VlanID       int
			L2Port       string
			EgressDevice string
			Description  string
			UpdatedAt    time.Time
		}
		a.DB.Table("ip_am_subnets").
			Select("ip_am_subnets.id, ip_am_subnets.network, ip_am_subnets.unit, ip_am_subnets.rate_mbps, ip_am_subnets.vlan_id, ip_am_subnets.l2_port, ip_am_subnets.egress_device, ip_am_subnets.description, ip_am_subnets.updated_at, ip_am_sections.name as section").
			Joins("left join ip_am_sections on ip_am_sections.id = ip_am_subnets.section_id").
			Where("ip_am_subnets.unit ILIKE ?", like).Find(&subRows)
		subnetList := make([]models.IPAMSubnet, 0, len(subRows))
		for _, row := range subRows {
			subnetList = append(subnetList, models.IPAMSubnet{ID: row.ID, Network: row.Network})
		}
		usedMap, _ := a.loadIPAMUsedCountsBySubnet(subnetList)
		for _, row := range subRows {
			util := buildSubnetUtil(row.Network, usedMap[row.ID])
			subnetMatches = append(subnetMatches, ipamSearchSubnet{
				ID:           row.ID,
				Network:      row.Network,
				Unit:         row.Unit,
				Section:      row.Section,
				RateMbps:     row.RateMbps,
				VlanID:       row.VlanID,
				L2Port:       row.L2Port,
				EgressDevice: row.EgressDevice,
				Description:  row.Description,
				UpdatedAt:    row.UpdatedAt.Format("2006-01-02 15:04"),
				UsedCount:    util.Used,
				TotalCount:   util.TotalText,
				UtilPercent:  util.PercentText,
			})
		}

		var addrRows []struct {
			ID           uint
			IP           string
			Status       string
			Unit         string
			Hostname     string
			Note         string
			SubnetID     uint
			Network      string
			Section      string
			RateMbps     int
			VlanID       int
			L2Port       string
			EgressDevice string
			UpdatedAt    time.Time
		}
		a.DB.Table("ip_am_addresses").
			Select("ip_am_addresses.id, ip_am_addresses.ip, ip_am_addresses.status, ip_am_addresses.unit, ip_am_addresses.hostname, ip_am_addresses.note, ip_am_addresses.subnet_id, ip_am_subnets.network, ip_am_sections.name as section, ip_am_subnets.rate_mbps, ip_am_subnets.vlan_id, ip_am_subnets.l2_port, ip_am_subnets.egress_device, ip_am_addresses.updated_at").
			Joins("left join ip_am_subnets on ip_am_subnets.id = ip_am_addresses.subnet_id").
			Joins("left join ip_am_sections on ip_am_sections.id = ip_am_subnets.section_id").
			Where("ip_am_addresses.unit ILIKE ?", like).Find(&addrRows)
		for _, row := range addrRows {
			statusLabel := ipamStatusLabel(row.Status)
			if strings.TrimSpace(row.Unit) != "" {
				statusLabel = "已使用"
			}
			addressMatches = append(addressMatches, ipamSearchAddress{
				ID:           row.ID,
				IP:           row.IP,
				Status:       statusLabel,
				Unit:         row.Unit,
				Hostname:     row.Hostname,
				Note:         row.Note,
				SubnetID:     row.SubnetID,
				Network:      row.Network,
				Section:      row.Section,
				RateMbps:     row.RateMbps,
				VlanID:       row.VlanID,
				L2Port:       row.L2Port,
				EgressDevice: row.EgressDevice,
				UpdatedAt:    row.UpdatedAt.Format("2006-01-02 15:04"),
			})
		}
	}

	return subnetMatches, addressMatches
}

func (a *AppContext) countUsedAddresses(subnetID uint) int64 {
	var count int64
	a.DB.Model(&models.IPAMAddress{}).Where("subnet_id = ? AND unit <> ''", subnetID).Count(&count)
	return count
}

func (a *AppContext) countUsedAddressesBySubnets(subnets []models.IPAMSubnet) int64 {
	if len(subnets) == 0 {
		return 0
	}
	ids := make([]uint, 0, len(subnets))
	for _, subnet := range subnets {
		ids = append(ids, subnet.ID)
	}
	var count int64
	a.DB.Model(&models.IPAMAddress{}).Where("subnet_id IN ? AND unit <> ''", ids).Count(&count)
	return count
}

type subnetUtil struct {
	Used        int64
	TotalText   string
	Percent     int
	PercentText string
	IsIPv6      bool
}

func buildSubnetUtil(network string, used int64) subnetUtil {
	prefix, err := netip.ParsePrefix(network)
	if err != nil {
		return subnetUtil{Used: used, TotalText: "-", PercentText: "-"}
	}
	if prefix.Addr().Is4() {
		total := ipv4Total(prefix)
		percent := 0
		percentText := "-"
		if total > 0 {
			percent = int(math.Round(float64(used) / float64(total) * 100))
			percentText = fmt.Sprintf("%d%%", percent)
		}
		return subnetUtil{Used: used, TotalText: fmt.Sprintf("%d", total), Percent: percent, PercentText: percentText}
	}
	total := ipv6Total(prefix)
	return subnetUtil{Used: used, TotalText: total, PercentText: "-", IsIPv6: true}
}

func ipv4Total(prefix netip.Prefix) int64 {
	bits := prefix.Bits()
	if bits < 0 || bits > 32 {
		return 0
	}
	return int64(1) << uint(32-bits)
}

func ipv6Total(prefix netip.Prefix) string {
	bits := prefix.Bits()
	if bits < 0 || bits > 128 {
		return "-"
	}
	pow := big.NewInt(1)
	pow.Lsh(pow, uint(128-bits))
	return pow.String()
}

func ipamStatusLabel(status string) string {
	for _, opt := range ipamStatusOptions {
		if opt.Value == status {
			return opt.Label
		}
	}
	if status == "" {
		return "已使用"
	}
	return status
}

func uintPtrToString(v *uint) string {
	if v == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*v), 10)
}

func parseUint(raw string) uint {
	if raw == "" {
		return 0
	}
	val, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return uint(val)
}

func findLargestFreePrefix(root netip.Prefix, subnets []models.IPAMSubnet) string {
	occupied := make([]netip.Prefix, 0, len(subnets))
	for _, subnet := range subnets {
		prefix, err := netip.ParsePrefix(subnet.Network)
		if err != nil {
			continue
		}
		if prefix.Addr().Is4() != root.Addr().Is4() {
			continue
		}
		if !prefixesOverlap(root, prefix) {
			continue
		}
		occupied = append(occupied, prefix)
	}
	free := freePrefixes(root, occupied)
	if len(free) == 0 {
		return ""
	}
	sort.Slice(free, func(i, j int) bool {
		return free[i].Bits() < free[j].Bits()
	})
	return free[0].String()
}

func freePrefixes(root netip.Prefix, occupied []netip.Prefix) []netip.Prefix {
	rel := filterOverlapping(root, occupied)
	if len(rel) == 0 {
		return []netip.Prefix{root}
	}
	for _, occ := range rel {
		if prefixContains(occ, root) {
			return nil
		}
	}
	left, right := splitPrefix(root)
	return append(freePrefixes(left, rel), freePrefixes(right, rel)...)
}

func freeBlocksForSubnet(root netip.Prefix, subnets []models.IPAMSubnet, limit int) []string {
	occupied := make([]netip.Prefix, 0, len(subnets))
	for _, subnet := range subnets {
		prefix, err := netip.ParsePrefix(subnet.Network)
		if err != nil {
			continue
		}
		if prefix == root {
			continue
		}
		if !prefixesOverlap(root, prefix) {
			continue
		}
		if !prefixContains(root, prefix) {
			continue
		}
		occupied = append(occupied, prefix)
	}
	free := freePrefixes(root, occupied)
	sort.Slice(free, func(i, j int) bool {
		if free[i].Bits() == free[j].Bits() {
			return free[i].String() < free[j].String()
		}
		return free[i].Bits() < free[j].Bits()
	})
	result := make([]string, 0, limit)
	for _, prefix := range free {
		result = append(result, prefix.String())
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

func buildBlockMap(prefix netip.Prefix, addresses []models.IPAMAddress, page int) ipamBlockMap {
	total := ipv4Total(prefix)
	if total <= 0 || total > 4096 {
		return ipamBlockMap{Note: "仅支持 IPv4 且规模不超过 4096 的子网展示"}
	}
	statusByIP := make(map[string]string)
	for _, addr := range addresses {
		status := addr.Status
		if strings.TrimSpace(addr.Unit) != "" {
			status = "used"
		}
		statusByIP[addr.IP] = status
	}
	pageSize := int64(256)
	pages := int(math.Ceil(float64(total) / float64(pageSize)))
	if page < 1 {
		page = 1
	}
	if page > pages {
		page = pages
	}
	startIndex := int64(page-1) * pageSize
	endIndex := startIndex + pageSize - 1
	if endIndex >= total {
		endIndex = total - 1
	}

	columns := 16
	if total < 16 {
		columns = int(total)
	}
	start, _ := prefixRange(prefix)
	cells := make([]ipamBlockCell, 0, int(endIndex-startIndex+1))
	for i := startIndex; i <= endIndex; i++ {
		ipVal := new(big.Int).Add(start, big.NewInt(i))
		ip := bigToAddr(ipVal, 32).String()
		status := statusByIP[ip]
		if status == "" {
			status = "free"
		}
		label := ipamStatusLabel(status)
		cells = append(cells, ipamBlockCell{
			IP:     ip,
			Status: status,
			Label:  label,
			Class:  ipamBlockClass(status),
		})
	}
	rangeText := fmt.Sprintf("%s - %s", bigToAddr(new(big.Int).Add(start, big.NewInt(startIndex)), 32).String(), bigToAddr(new(big.Int).Add(start, big.NewInt(endIndex)), 32).String())
	return ipamBlockMap{
		Columns: columns,
		Cells:   cells,
		Note:    "",
		Page:    page,
		Total:   total,
		Pages:   pages,
		Range:   rangeText,
		PrevPage: func() int {
			if page > 1 {
				return page - 1
			}
			return 1
		}(),
		NextPage: func() int {
			if page < pages {
				return page + 1
			}
			return pages
		}(),
		HasPrev: page > 1,
		HasNext: page < pages,
	}
}

func buildMapSpace(prefix netip.Prefix, subnets []models.IPAMSubnet, util subnetUtil, maxBits int, collapseUsed bool, linkByCIDR map[string]string) ipamMapSpace {
	if !prefix.Addr().Is4() {
		return ipamMapSpace{Enabled: false, Note: "IPv6 暂不支持地图空间"}
	}
	occupied := make([]netip.Prefix, 0, len(subnets))
	subnetLinks := buildSubnetLinks(prefix, subnets)
	for _, subnet := range subnets {
		p, err := netip.ParsePrefix(subnet.Network)
		if err != nil {
			continue
		}
		if p == prefix {
			continue
		}
		if prefixContains(prefix, p) {
			occupied = append(occupied, p)
		}
	}
	rows := buildMapRows(prefix, occupied, maxBits, collapseUsed, linkByCIDR, subnetLinks)
	return ipamMapSpace{
		Enabled:       len(rows) > 0,
		Rows:          rows,
		UtilPercent:   util.Percent,
		UtilText:      util.PercentText,
		UsedAddresses: util.Used,
		TotalText:     util.TotalText,
		Note:          "",
	}
}

func buildMapRows(prefix netip.Prefix, occupied []netip.Prefix, maxBits int, collapseUsed bool, linkByCIDR map[string]string, subnetLinks []ipamSubnetLink) []ipamMapRow {
	rows := []ipamMapRow{}
	baseBits := prefix.Bits()
	if baseBits >= 32 {
		return rows
	}
	if maxBits <= 0 || maxBits > 30 {
		maxBits = 30
	}
	limit := 128
	if maxBits <= 24 {
		limit = 256
	}
	for bits := baseBits + 1; bits <= maxBits; bits++ {
		offset := bits - baseBits
		if offset <= 0 {
			continue
		}
		total := 1 << offset
		if total > limit {
			break
		}
		blocks := splitPrefixList(prefix, bits, total)
		rowBlocks := make([]ipamMapBlock, 0, total)
		usedCount := 0
		for _, block := range blocks {
			used, skip := mapBlockUsage(block, occupied, collapseUsed)
			if skip {
				continue
			}
			if used {
				usedCount++
			}
			link := ""
			if linkByCIDR != nil {
				if val, ok := linkByCIDR[block.String()]; ok {
					link = val
				}
			}
			if link == "" && used {
				if id, ok := findSmallestSubnetLink(block, subnetLinks); ok {
					link = fmt.Sprintf("/ipam/subnets/%d", id)
				}
			}
			rowBlocks = append(rowBlocks, ipamMapBlock{
				CIDR: block.String(),
				Used: used,
				Link: link,
			})
		}
		if collapseUsed {
			total = len(rowBlocks)
		}
		if total == 0 {
			continue
		}
		free := total - usedCount
		if free < 0 {
			free = 0
		}
		rows = append(rows, ipamMapRow{
			PrefixLen: bits,
			Used:      usedCount,
			Free:      free,
			Total:     total,
			Blocks:    rowBlocks,
		})
	}
	return rows
}

func mapBlockUsage(block netip.Prefix, occupied []netip.Prefix, collapseUsed bool) (bool, bool) {
	for _, occ := range occupied {
		if !prefixesOverlap(block, occ) {
			continue
		}
		if prefixContains(occ, block) {
			if collapseUsed && occ.Bits() < block.Bits() {
				return false, true
			}
			return true, false
		}
		return true, false
	}
	return false, false
}

type ipamSubnetLink struct {
	ID     uint
	Prefix netip.Prefix
}

func buildSubnetLinks(root netip.Prefix, subnets []models.IPAMSubnet) []ipamSubnetLink {
	result := make([]ipamSubnetLink, 0, len(subnets))
	for _, subnet := range subnets {
		p, err := netip.ParsePrefix(subnet.Network)
		if err != nil {
			continue
		}
		if root.Addr().Is4() != p.Addr().Is4() {
			continue
		}
		if !prefixContains(root, p) {
			continue
		}
		result = append(result, ipamSubnetLink{ID: subnet.ID, Prefix: p})
	}
	return result
}

func findSmallestSubnetLink(block netip.Prefix, links []ipamSubnetLink) (uint, bool) {
	bestBits := -1
	var bestID uint
	for _, link := range links {
		if !prefixContains(link.Prefix, block) {
			continue
		}
		if link.Prefix.Bits() > bestBits {
			bestBits = link.Prefix.Bits()
			bestID = link.ID
		}
	}
	if bestBits == -1 {
		return 0, false
	}
	return bestID, true
}

func ipamBlockClass(status string) string {
	switch status {
	case "used":
		return "bg-emerald-500"
	case "reserved":
		return "bg-amber-400"
	case "dhcp":
		return "bg-blue-500"
	case "free":
		return "bg-slate-300 dark:bg-slate-700"
	default:
		return "bg-slate-300 dark:bg-slate-700"
	}
}

func filterOverlapping(root netip.Prefix, occupied []netip.Prefix) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(occupied))
	for _, p := range occupied {
		if prefixesOverlap(root, p) {
			out = append(out, p)
		}
	}
	return out
}

func prefixContains(container, candidate netip.Prefix) bool {
	return container.Contains(candidate.Addr()) && container.Bits() <= candidate.Bits()
}

func prefixesOverlap(a, b netip.Prefix) bool {
	startA, endA := prefixRange(a)
	startB, endB := prefixRange(b)
	return startA.Cmp(endB) <= 0 && startB.Cmp(endA) <= 0
}

func splitPrefix(prefix netip.Prefix) (netip.Prefix, netip.Prefix) {
	bits := prefix.Bits()
	addrBits := 128
	if prefix.Addr().Is4() {
		addrBits = 32
	}
	childBits := bits + 1
	if childBits > addrBits {
		return prefix, prefix
	}
	start := addrToBig(prefix.Addr())
	offset := big.NewInt(1)
	offset.Lsh(offset, uint(addrBits-childBits))
	secondStart := big.NewInt(0).Add(start, offset)
	firstAddr := bigToAddr(start, addrBits)
	secondAddr := bigToAddr(secondStart, addrBits)
	return netip.PrefixFrom(firstAddr, childBits), netip.PrefixFrom(secondAddr, childBits)
}

func splitPrefixList(prefix netip.Prefix, newBits int, count int) []netip.Prefix {
	addrBits := 128
	if prefix.Addr().Is4() {
		addrBits = 32
	}
	start := addrToBig(prefix.Addr())
	step := big.NewInt(1)
	step.Lsh(step, uint(addrBits-newBits))
	items := make([]netip.Prefix, 0, count)
	current := new(big.Int).Set(start)
	for i := 0; i < count; i++ {
		addr := bigToAddr(current, addrBits)
		items = append(items, netip.PrefixFrom(addr, newBits))
		current = new(big.Int).Add(current, step)
	}
	return items
}

func prefixRange(prefix netip.Prefix) (*big.Int, *big.Int) {
	addrBits := 128
	if prefix.Addr().Is4() {
		addrBits = 32
	}
	start := addrToBig(prefix.Addr())
	size := big.NewInt(1)
	size.Lsh(size, uint(addrBits-prefix.Bits()))
	end := big.NewInt(0).Add(start, size)
	end.Sub(end, big.NewInt(1))
	return start, end
}

func addrToBig(addr netip.Addr) *big.Int {
	bytes := addr.As16()
	if addr.Is4() {
		bytes = [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, bytes[12], bytes[13], bytes[14], bytes[15]}
	}
	return new(big.Int).SetBytes(bytes[:])
}

func bigToAddr(value *big.Int, bits int) netip.Addr {
	bytes := value.Bytes()
	buf := make([]byte, 16)
	copy(buf[16-len(bytes):], bytes)
	if bits == 32 {
		return netip.AddrFrom4([4]byte{buf[12], buf[13], buf[14], buf[15]})
	}
	return netip.AddrFrom16([16]byte{buf[0], buf[1], buf[2], buf[3], buf[4], buf[5], buf[6], buf[7], buf[8], buf[9], buf[10], buf[11], buf[12], buf[13], buf[14], buf[15]})
}

func (a *AppContext) validateSubnetParent(record models.IPAMSubnet) error {
	if record.ParentID == nil {
		roots, err := a.loadIPAMCategoryRoots([]uint{record.SectionID})
		if err != nil {
			return fmt.Errorf("读取根网段失败")
		}
		rootList := buildRootCIDRList(roots[record.SectionID], "")
		if len(rootList) == 0 {
			return nil
		}
		childPrefix, err := netip.ParsePrefix(record.Network)
		if err != nil {
			return fmt.Errorf("子网 CIDR 无效")
		}
		for _, root := range rootList {
			rootPrefix, err := netip.ParsePrefix(root)
			if err != nil {
				continue
			}
			if prefixContains(rootPrefix, childPrefix) {
				return nil
			}
		}
		return fmt.Errorf("子网不在类别根网段范围内")
	}
	var parent models.IPAMSubnet
	if err := a.DB.First(&parent, *record.ParentID).Error; err != nil {
		return fmt.Errorf("父子网不存在")
	}
	if parent.SectionID != record.SectionID {
		return fmt.Errorf("父子网所属类别不一致")
	}
	parentPrefix, err := netip.ParsePrefix(parent.Network)
	if err != nil {
		return fmt.Errorf("父子网 CIDR 无效")
	}
	childPrefix, err := netip.ParsePrefix(record.Network)
	if err != nil {
		return fmt.Errorf("子网 CIDR 无效")
	}
	if !prefixContains(parentPrefix, childPrefix) {
		return fmt.Errorf("子网不在父子网范围内")
	}
	return nil
}

func ipamWriteErrorMessage(prefix string, err error) string {
	if isIPAMOverlapError(err) {
		return prefix + "地址段与已有记录重叠"
	}
	return prefix + err.Error()
}

func isIPAMOverlapError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "ipam_subnets_no_overlap") || strings.Contains(msg, "exclusion") || strings.Contains(msg, "overlaps")
}

func parseIPAMPrefix(raw string) (netip.Prefix, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return netip.Prefix{}, fmt.Errorf("CIDR 不能为空")
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("CIDR 格式错误")
	}
	prefix = prefix.Masked()
	bits := prefix.Bits()
	if prefix.Addr().Is4() {
		if bits < 8 || bits > 32 {
			return netip.Prefix{}, fmt.Errorf("IPv4 前缀长度需在 /8 到 /32 之间")
		}
		return prefix, nil
	}
	if prefix.Addr().Is6() {
		if bits < 64 || bits > 128 {
			return netip.Prefix{}, fmt.Errorf("IPv6 前缀长度需在 /64 到 /128 之间")
		}
		return prefix, nil
	}
	return netip.Prefix{}, fmt.Errorf("IP 版本不支持")
}

func parsePositiveInt(raw, field string, min, max int) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("%s不能为空", field)
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s格式错误", field)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("%s需在 %d 到 %d 之间", field, min, max)
	}
	return n, nil
}

func normalizeIPAMStatus(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	for _, opt := range ipamStatusOptions {
		if opt.Value == value {
			return value
		}
	}
	return "used"
}

func newCSVReader(r io.Reader) *csv.Reader {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	return reader
}

func newCSVWriter(w io.Writer) *csv.Writer {
	writer := csv.NewWriter(w)
	writer.UseCRLF = false
	return writer
}

func writeUTF8BOM(w io.Writer) {
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
}

func parseIPAMCSV(reader io.Reader) ([]ipamImportRow, error) {
	csvReader := newCSVReader(reader)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	rows := make([]ipamImportRow, 0, len(records))
	for idx, row := range records {
		if len(row) == 0 {
			continue
		}
		ip := strings.TrimSpace(row[0])
		if idx == 0 && strings.EqualFold(ip, "ip") {
			continue
		}
		entry := ipamImportRow{
			IP: ip,
		}
		if len(row) > 1 {
			entry.Status = normalizeIPAMStatus(strings.TrimSpace(row[1]))
		} else {
			entry.Status = "used"
		}
		if len(row) > 2 {
			entry.Unit = strings.TrimSpace(row[2])
		}
		if len(row) > 3 {
			entry.Hostname = strings.TrimSpace(row[3])
		}
		if len(row) > 4 {
			entry.Note = strings.TrimSpace(row[4])
		}
		if entry.IP == "" {
			entry.Error = "IP 不能为空"
		} else if _, err := netip.ParseAddr(entry.IP); err != nil {
			entry.Error = "IP 格式错误"
		}
		rows = append(rows, entry)
	}
	return rows, nil
}

func buildImportResult(rows []ipamImportRow) ipamImportResult {
	result := ipamImportResult{}
	for _, row := range rows {
		result.Total++
		if row.Error != "" {
			result.Invalid++
			if len(result.Errors) < 10 {
				result.Errors = append(result.Errors, fmt.Sprintf("%s：%s", row.IP, row.Error))
			}
			continue
		}
		result.Valid++
		if len(result.Samples) < 8 {
			result.Samples = append(result.Samples, row)
		}
	}
	return result
}

func applyImportRows(a *AppContext, subnet models.IPAMSubnet, rows []ipamImportRow) ipamImportResult {
	result := buildImportResult(rows)
	for _, row := range rows {
		if row.Error != "" {
			result.Skipped++
			continue
		}
		var existing models.IPAMAddress
		err := a.DB.Where("subnet_id = ? AND ip = ?", subnet.ID, row.IP).First(&existing).Error
		if err == nil {
			existing.Status = row.Status
			existing.Unit = row.Unit
			existing.Hostname = row.Hostname
			existing.Note = row.Note
			existing.UpdatedAt = time.Now()
			if err := a.DB.Save(&existing).Error; err == nil {
				result.Updated++
			} else {
				result.Skipped++
			}
			continue
		}
		record := models.IPAMAddress{
			SubnetID: subnet.ID,
			IP:       row.IP,
			Status:   row.Status,
			Unit:     row.Unit,
			Hostname: row.Hostname,
			Note:     row.Note,
		}
		if err := a.DB.Create(&record).Error; err == nil {
			result.Created++
		} else {
			result.Skipped++
		}
	}
	return result
}

func parseIPAMSubnetCSV(reader io.Reader) ([]ipamSubnetImportRow, error) {
	csvReader := newCSVReader(reader)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return []ipamSubnetImportRow{}, nil
	}

	headerMap, hasHeader := parseSubnetCSVHeader(records[0])
	start := 0
	if hasHeader {
		start = 1
	}

	rows := make([]ipamSubnetImportRow, 0, len(records))
	for idx := start; idx < len(records); idx++ {
		row := records[idx]
		if len(row) == 0 {
			continue
		}
		entry := ipamSubnetImportRow{
			RootCIDR:    readSubnetCSVCell(row, headerMap, "root", 0),
			Subnet:      readSubnetCSVCell(row, headerMap, "subnet", 1),
			Unit:        readSubnetCSVCell(row, headerMap, "unit", 2),
			RateMbps:    readSubnetCSVCell(row, headerMap, "rate", 3),
			VlanID:      readSubnetCSVCell(row, headerMap, "vlan", 4),
			L2Port:      readSubnetCSVCell(row, headerMap, "l2", 5),
			Device:      readSubnetCSVCell(row, headerMap, "device", 6),
			Description: readSubnetCSVCell(row, headerMap, "description", 7),
		}
		entry.RootCIDR = strings.TrimSpace(entry.RootCIDR)
		entry.Subnet = strings.TrimSpace(entry.Subnet)
		entry.Unit = strings.TrimSpace(entry.Unit)
		entry.RateMbps = strings.TrimSpace(entry.RateMbps)
		entry.VlanID = strings.TrimSpace(entry.VlanID)
		entry.L2Port = strings.TrimSpace(entry.L2Port)
		entry.Device = strings.TrimSpace(entry.Device)
		entry.Description = strings.TrimSpace(entry.Description)

		if entry.Subnet == "" {
			entry.Error = "子网不能为空"
			rows = append(rows, entry)
			continue
		}
		if _, err := parseIPAMPrefix(entry.Subnet); err != nil {
			entry.Error = "子网格式错误：" + err.Error()
			rows = append(rows, entry)
			continue
		}
		if entry.RootCIDR != "" {
			if _, err := parseIPAMPrefix(entry.RootCIDR); err != nil {
				entry.Error = "根网段格式错误：" + err.Error()
				rows = append(rows, entry)
				continue
			}
		}
		if _, err := parseImportInt(entry.RateMbps); err != nil {
			entry.Error = "速率格式错误"
			rows = append(rows, entry)
			continue
		}
		if _, err := parseImportInt(entry.VlanID); err != nil {
			entry.Error = "VLAN格式错误"
			rows = append(rows, entry)
			continue
		}
		rows = append(rows, entry)
	}
	return rows, nil
}

func parseSubnetCSVHeader(row []string) (map[string]int, bool) {
	if len(row) == 0 {
		return nil, false
	}
	header := make(map[string]int)
	for idx, cell := range row {
		key := normalizeSubnetHeader(cell)
		if key == "" {
			continue
		}
		if field, ok := subnetHeaderAlias[key]; ok {
			header[field] = idx
		}
	}
	return header, len(header) > 0
}

func readSubnetCSVCell(row []string, header map[string]int, field string, fallback int) string {
	if header != nil {
		if idx, ok := header[field]; ok {
			if idx >= 0 && idx < len(row) {
				return row[idx]
			}
			return ""
		}
	}
	if fallback >= 0 && fallback < len(row) {
		return row[fallback]
	}
	return ""
}

func normalizeSubnetHeader(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ToLower(trimmed)
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", "（", "", "）", "", "(", "", ")", "", "：", "", ":", "")
	return replacer.Replace(normalized)
}

var subnetHeaderAlias = map[string]string{
	"root":         "root",
	"rootcidr":     "root",
	"根网段":          "root",
	"subnet":       "subnet",
	"cidr":         "subnet",
	"network":      "subnet",
	"网段":           "subnet",
	"子网":           "subnet",
	"ip段":          "subnet",
	"unit":         "unit",
	"客户单位名称":       "unit",
	"使用单位":         "unit",
	"客户单位":         "unit",
	"rate":         "rate",
	"ratembps":     "rate",
	"速率":           "rate",
	"带宽":           "rate",
	"vlan":         "vlan",
	"vlanid":       "vlan",
	"l2":           "l2",
	"l2port":       "l2",
	"二层节点":         "l2",
	"二层端口":         "l2",
	"二层节点端口":       "l2",
	"device":       "device",
	"egressdevice": "device",
	"三层设备":         "device",
	"出口设备":         "device",
	"description":  "description",
	"note":         "description",
	"备注":           "description",
	"说明":           "description",
}

func parseImportInt(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	num, err := strconv.Atoi(trimmed)
	if err != nil || num < 0 {
		return 0, fmt.Errorf("invalid")
	}
	return num, nil
}

func buildSubnetImportResult(rows []ipamSubnetImportRow) ipamSubnetImportResult {
	result := ipamSubnetImportResult{}
	for _, row := range rows {
		result.Total++
		if row.Error != "" {
			result.Invalid++
			if len(result.Errors) < 10 {
				result.Errors = append(result.Errors, fmt.Sprintf("%s：%s", row.Subnet, row.Error))
			}
			continue
		}
		result.Valid++
		if len(result.Samples) < 8 {
			result.Samples = append(result.Samples, row)
		}
	}
	return result
}

func applySubnetImportRows(a *AppContext, section models.IPAMSection, rows []ipamSubnetImportRow) ipamSubnetImportResult {
	result := buildSubnetImportResult(rows)
	if result.Valid == 0 {
		return result
	}
	importedSet := make(map[string]ipamSubnetImportRow)
	importedPrefixes := make([]netip.Prefix, 0, result.Valid)
	rootSet := make(map[string]struct{})
	for _, row := range rows {
		if row.Error != "" {
			continue
		}
		prefix, err := parseIPAMPrefix(row.Subnet)
		if err != nil {
			continue
		}
		key := prefix.String()
		row.Subnet = key
		importedSet[key] = row
		importedPrefixes = append(importedPrefixes, prefix)
		if row.RootCIDR != "" {
			if rootPrefix, err := parseIPAMPrefix(row.RootCIDR); err == nil {
				rootSet[rootPrefix.String()] = struct{}{}
			}
		}
	}

	for root := range rootSet {
		if err := a.ensureSectionRootFromImport(section.ID, root); err != nil && len(result.Errors) < 10 {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	var existing []models.IPAMSubnet
	_ = a.DB.Where("section_id = ?", section.ID).Find(&existing).Error
	toDelete := make([]uint, 0)
	for _, record := range existing {
		if _, ok := importedSet[record.Network]; ok {
			continue
		}
		recordPrefix, err := netip.ParsePrefix(record.Network)
		if err != nil {
			continue
		}
		for _, imported := range importedPrefixes {
			if prefixesOverlap(recordPrefix, imported) {
				toDelete = append(toDelete, record.ID)
				break
			}
		}
	}
	if len(toDelete) > 0 {
		_ = a.DB.Where("subnet_id IN ?", toDelete).Delete(&models.IPAMAddress{}).Error
		_ = a.DB.Where("id IN ?", toDelete).Delete(&models.IPAMSubnet{}).Error
		result.Deleted = len(toDelete)
	}

	for _, row := range importedSet {
		rate, _ := parseImportInt(row.RateMbps)
		vlan, _ := parseImportInt(row.VlanID)
		var existingSubnet models.IPAMSubnet
		err := a.DB.Where("section_id = ? AND network = ?", section.ID, row.Subnet).First(&existingSubnet).Error
		if err == nil {
			existingSubnet.Unit = row.Unit
			existingSubnet.RateMbps = rate
			existingSubnet.VlanID = vlan
			existingSubnet.L2Port = row.L2Port
			existingSubnet.EgressDevice = row.Device
			existingSubnet.Description = row.Description
			existingSubnet.UpdatedAt = time.Now()
			if err := a.DB.Save(&existingSubnet).Error; err == nil {
				result.Updated++
			}
			continue
		}
		record := models.IPAMSubnet{
			SectionID:    section.ID,
			Network:      row.Subnet,
			Unit:         row.Unit,
			VRF:          "default",
			RateMbps:     rate,
			VlanID:       vlan,
			L2Port:       row.L2Port,
			EgressDevice: row.Device,
			Description:  row.Description,
		}
		if err := a.DB.Create(&record).Error; err == nil {
			result.Created++
		}
	}
	return result
}

func (a *AppContext) generateAddressPool(subnet models.IPAMSubnet, form ipamGenerateForm, validateOnly bool) (ipamGenerateResult, error) {
	result := ipamGenerateResult{}
	if form.StartIP == "" {
		return result, fmt.Errorf("起始 IP 不能为空")
	}
	if form.EndIP == "" && strings.TrimSpace(form.Count) == "" {
		return result, fmt.Errorf("请填写结束 IP 或生成数量")
	}
	start, err := netip.ParseAddr(form.StartIP)
	if err != nil {
		return result, fmt.Errorf("起始 IP 格式错误")
	}
	var end netip.Addr
	if form.EndIP != "" {
		end, err = netip.ParseAddr(form.EndIP)
		if err != nil {
			return result, fmt.Errorf("结束 IP 格式错误")
		}
		if start.Is4() != end.Is4() {
			return result, fmt.Errorf("起始与结束 IP 版本不一致")
		}
	} else {
		count, err := strconv.Atoi(form.Count)
		if err != nil || count <= 0 {
			return result, fmt.Errorf("生成数量格式错误")
		}
		addrBits := 32
		if start.Is6() {
			addrBits = 128
		}
		startInt := addrToBig(start)
		endInt := new(big.Int).Add(startInt, big.NewInt(int64(count-1)))
		end = bigToAddr(endInt, addrBits)
	}
	prefix, err := netip.ParsePrefix(subnet.Network)
	if err != nil {
		return result, fmt.Errorf("子网 CIDR 无效")
	}
	if !prefix.Contains(start) || !prefix.Contains(end) {
		return result, fmt.Errorf("IP 范围不在子网内")
	}
	startInt := addrToBig(start)
	endInt := addrToBig(end)
	if startInt.Cmp(endInt) > 0 {
		return result, fmt.Errorf("起始 IP 不能大于结束 IP")
	}
	diff := new(big.Int).Sub(endInt, startInt)
	diff.Add(diff, big.NewInt(1))

	maxCount := int64(4096)
	if start.Is6() {
		maxCount = 1024
	}
	if diff.Cmp(big.NewInt(maxCount)) > 0 {
		return result, fmt.Errorf("生成数量过多，最大支持 %d 条", maxCount)
	}
	count := int(diff.Int64())
	result.Total = count
	result.Valid = count

	addrBits := 32
	if start.Is6() {
		addrBits = 128
	}
	for i := 0; i < count; i++ {
		ipVal := new(big.Int).Add(startInt, big.NewInt(int64(i)))
		ip := bigToAddr(ipVal, addrBits).String()
		if len(result.Samples) < 8 {
			result.Samples = append(result.Samples, ip)
		}
		if validateOnly {
			continue
		}
		var existing models.IPAMAddress
		if err := a.DB.Where("subnet_id = ? AND ip = ?", subnet.ID, ip).First(&existing).Error; err == nil {
			result.Skipped++
			continue
		}
		record := models.IPAMAddress{
			SubnetID: subnet.ID,
			IP:       ip,
			Status:   form.Status,
			Unit:     form.Unit,
			Hostname: buildHostname(form.HostnamePrefix, i),
			Note:     form.Note,
		}
		if err := a.DB.Create(&record).Error; err == nil {
			result.Created++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}

func buildHostname(prefix string, idx int) string {
	if strings.TrimSpace(prefix) == "" {
		return ""
	}
	return fmt.Sprintf("%s-%d", prefix, idx+1)
}

func parsePage(raw string) int {
	val, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || val < 1 {
		return 1
	}
	return val
}

func (a *AppContext) ensureSectionRootFromForm(sectionID uint, rootCIDR string) error {
	rootCIDR = strings.TrimSpace(rootCIDR)
	if rootCIDR == "" {
		return nil
	}
	prefix, err := parseIPAMPrefix(rootCIDR)
	if err != nil {
		return err
	}

	var count int64
	if err := a.DB.Model(&models.IPAMCategoryRoot{}).Where("category_id = ? AND cidr = ?", sectionID, prefix.String()).Count(&count).Error; err == nil && count > 0 {
		return nil
	}
	record := models.IPAMCategoryRoot{
		CategoryID: sectionID,
		CIDR:       prefix.String(),
		Note:       "来自类别表单",
	}
	if err := a.DB.Create(&record).Error; err != nil {
		return fmt.Errorf("新增根网段失败：%w", err)
	}
	return nil
}

func (a *AppContext) ensureSectionRootFromImport(sectionID uint, rootCIDR string) error {
	rootCIDR = strings.TrimSpace(rootCIDR)
	if rootCIDR == "" {
		return nil
	}
	prefix, err := parseIPAMPrefix(rootCIDR)
	if err != nil {
		return err
	}
	roots, err := a.loadIPAMCategoryRoots([]uint{sectionID})
	if err == nil {
		for _, root := range roots[sectionID] {
			existing, err := netip.ParsePrefix(root.CIDR)
			if err != nil {
				continue
			}
			if existing.Addr().Is4() != prefix.Addr().Is4() {
				continue
			}
			if prefixesOverlap(existing, prefix) {
				return fmt.Errorf("根网段与现有范围重叠：%s", prefix.String())
			}
		}
	}
	var count int64
	if err := a.DB.Model(&models.IPAMCategoryRoot{}).Where("category_id = ? AND cidr = ?", sectionID, prefix.String()).Count(&count).Error; err == nil && count > 0 {
		return nil
	}
	record := models.IPAMCategoryRoot{
		CategoryID: sectionID,
		CIDR:       prefix.String(),
		Note:       "来自批量导入",
	}
	if err := a.DB.Create(&record).Error; err != nil {
		return fmt.Errorf("新增根网段失败：%w", err)
	}
	return nil
}

func renderIPAMError(c *gin.Context, title, path string, err error) {
	if err == nil {
		return
	}
	c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
		"Title":   title,
		"Path":    path,
		"Message": err.Error(),
	})
}
