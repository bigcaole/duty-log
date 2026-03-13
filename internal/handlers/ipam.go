package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/netip"
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
	RootCIDR        string
	Description     string
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
	ID      uint
	Network string
	Unit    string
	VRF     string
	Section string
}

type ipamSearchAddress struct {
	ID        uint
	IP        string
	Status    string
	Unit      string
	Hostname  string
	SubnetID  uint
	Network   string
	Section   string
	UpdatedAt string
}

type ipamSectionForm struct {
	ID          uint
	Name        string
	RootCIDR    string
	Description string
}

type ipamSubnetItem struct {
	ID           uint
	Network      string
	Unit         string
	VRF          string
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
}

type ipamSubnetForm struct {
	ID           uint
	SectionID    uint
	ParentID     string
	Network      string
	Unit         string
	VRF          string
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
	group.GET("/ipam/sections/:id/edit", app.ipamSectionEditPage)
	group.POST("/ipam/sections/:id/edit", app.ipamSectionUpdate)
	group.POST("/ipam/sections/:id/delete", app.ipamSectionDelete)

	group.GET("/ipam/subnets/create", app.ipamSubnetCreatePage)
	group.POST("/ipam/subnets/create", app.ipamSubnetCreate)
	group.GET("/ipam/subnets/:id", app.ipamSubnetDetail)
	group.GET("/ipam/subnets/:id/edit", app.ipamSubnetEditPage)
	group.POST("/ipam/subnets/:id/edit", app.ipamSubnetUpdate)
	group.POST("/ipam/subnets/:id/delete", app.ipamSubnetDelete)
	group.GET("/ipam/subnets/:id/split", app.ipamSubnetSplitPage)
	group.POST("/ipam/subnets/:id/split", app.ipamSubnetSplit)

	group.GET("/ipam/subnets/:id/addresses", app.ipamAddressList)
	group.GET("/ipam/subnets/:id/addresses/create", app.ipamAddressCreatePage)
	group.POST("/ipam/subnets/:id/addresses/create", app.ipamAddressCreate)
	group.GET("/ipam/subnets/:id/addresses/import", app.ipamAddressImportPage)
	group.POST("/ipam/subnets/:id/addresses/import", app.ipamAddressImport)
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

	items := a.buildSectionItems(sections, subnets, addressCounts)

	var subnetMatches []ipamSearchSubnet
	var addressMatches []ipamSearchAddress
	if searchIP != "" || searchUnit != "" {
		subnetMatches, addressMatches = a.searchIPAM(searchIP, searchUnit)
	}

	c.HTML(http.StatusOK, "ipam/list.html", gin.H{
		"Title":           "IPAM 资产管理",
		"Sections":        items,
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
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可创建大区")
		return
	}
	c.HTML(http.StatusOK, "ipam/section_form.html", gin.H{
		"Title":  "新建 IPAM 大区",
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
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可创建大区")
		return
	}

	form := ipamSectionForm{
		Name:        strings.TrimSpace(c.PostForm("name")),
		RootCIDR:    strings.TrimSpace(c.PostForm("root_cidr")),
		Description: strings.TrimSpace(c.PostForm("description")),
	}
	if form.Name == "" {
		c.HTML(http.StatusBadRequest, "ipam/section_form.html", gin.H{
			"Title":  "新建 IPAM 大区",
			"Action": "/ipam/sections/create",
			"Form":   form,
			"Error":  "大区名称不能为空",
		})
		return
	}
	if form.RootCIDR != "" {
		if _, err := parseIPAMPrefix(form.RootCIDR); err != nil {
			c.HTML(http.StatusBadRequest, "ipam/section_form.html", gin.H{
				"Title":  "新建 IPAM 大区",
				"Action": "/ipam/sections/create",
				"Form":   form,
				"Error":  err.Error(),
			})
			return
		}
	}

	section := models.IPAMSection{
		Name:        form.Name,
		RootCIDR:    form.RootCIDR,
		Description: form.Description,
	}
	if err := a.DB.Create(&section).Error; err != nil {
		c.HTML(http.StatusBadRequest, "ipam/section_form.html", gin.H{
			"Title":  "新建 IPAM 大区",
			"Action": "/ipam/sections/create",
			"Form":   form,
			"Error":  "创建失败：" + err.Error(),
		})
		return
	}
	c.Redirect(http.StatusFound, "/ipam?msg=大区创建成功")
}

func (a *AppContext) ipamSectionDetail(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效大区ID")
		return
	}

	var section models.IPAMSection
	if err := a.DB.First(&section, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=大区不存在")
		return
	}

	subnets, err := a.loadIPAMSubnetsBySection([]uint{section.ID})
	if err != nil {
		renderIPAMError(c, "IPAM 大区", "/ipam", err)
		return
	}
	addressCounts, err := a.loadIPAMUsedCountsBySubnet(subnets)
	if err != nil {
		renderIPAMError(c, "IPAM 大区", "/ipam", err)
		return
	}
	items := buildSubnetTreeItems(subnets, addressCounts)

	maxFree := "未配置"
	if section.RootCIDR != "" {
		if prefix, err := parseIPAMPrefix(section.RootCIDR); err == nil {
			freePrefix, vrf := findLargestFreePrefixByVRF(prefix, subnets)
			if freePrefix == "" {
				maxFree = "无空闲"
			} else if vrf != "" {
				maxFree = fmt.Sprintf("%s（VRF:%s）", freePrefix, vrf)
			} else {
				maxFree = freePrefix
			}
		}
	}

	c.HTML(http.StatusOK, "ipam/section_detail.html", gin.H{
		"Title":     "IPAM 大区详情",
		"Section":   section,
		"Items":     items,
		"CanManage": currentUser.IsAdmin,
		"MaxFree":   maxFree,
	})
}

func (a *AppContext) ipamSectionEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可编辑大区")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效大区ID")
		return
	}

	var section models.IPAMSection
	if err := a.DB.First(&section, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=大区不存在")
		return
	}

	form := ipamSectionForm{
		ID:          section.ID,
		Name:        section.Name,
		RootCIDR:    section.RootCIDR,
		Description: section.Description,
	}
	c.HTML(http.StatusOK, "ipam/section_form.html", gin.H{
		"Title":  "编辑 IPAM 大区",
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
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可编辑大区")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效大区ID")
		return
	}

	var section models.IPAMSection
	if err := a.DB.First(&section, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=大区不存在")
		return
	}

	form := ipamSectionForm{
		ID:          section.ID,
		Name:        strings.TrimSpace(c.PostForm("name")),
		RootCIDR:    strings.TrimSpace(c.PostForm("root_cidr")),
		Description: strings.TrimSpace(c.PostForm("description")),
	}
	if form.Name == "" {
		c.HTML(http.StatusBadRequest, "ipam/section_form.html", gin.H{
			"Title":  "编辑 IPAM 大区",
			"Action": fmt.Sprintf("/ipam/sections/%d/edit", section.ID),
			"Form":   form,
			"Error":  "大区名称不能为空",
		})
		return
	}
	if form.RootCIDR != "" {
		if _, err := parseIPAMPrefix(form.RootCIDR); err != nil {
			c.HTML(http.StatusBadRequest, "ipam/section_form.html", gin.H{
				"Title":  "编辑 IPAM 大区",
				"Action": fmt.Sprintf("/ipam/sections/%d/edit", section.ID),
				"Form":   form,
				"Error":  err.Error(),
			})
			return
		}
	}

	section.Name = form.Name
	section.RootCIDR = form.RootCIDR
	section.Description = form.Description
	section.UpdatedAt = time.Now()
	if err := a.DB.Save(&section).Error; err != nil {
		c.HTML(http.StatusBadRequest, "ipam/section_form.html", gin.H{
			"Title":  "编辑 IPAM 大区",
			"Action": fmt.Sprintf("/ipam/sections/%d/edit", section.ID),
			"Form":   form,
			"Error":  "更新失败：" + err.Error(),
		})
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
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可删除大区")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效大区ID")
		return
	}

	if err := a.DB.Delete(&models.IPAMSection{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/ipam?msg=大区已删除")
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

	if overlap, err := a.ipamHasOverlap(record.Network, 0, record.SectionID, record.VRF, record.ParentID); err != nil {
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

	usedCount := a.countUsedAddresses(record.ID)
	util := buildSubnetUtil(record.Network, usedCount)
	freeBlocks := []string{}
	if prefix, err := netip.ParsePrefix(record.Network); err == nil {
		var siblings []models.IPAMSubnet
		if err := a.DB.Where("section_id = ? AND vrf = ?", record.SectionID, record.VRF).Find(&siblings).Error; err == nil {
			freeBlocks = freeBlocksForSubnet(prefix, siblings, 12)
		}
	}

	c.HTML(http.StatusOK, "ipam/subnet_detail.html", gin.H{
		"Title":      "子网详情",
		"Record":     record,
		"Util":       util,
		"FreeBlocks": freeBlocks,
		"CanManage":  currentUser.IsAdmin,
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
		VRF:          record.VRF,
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

	if overlap, err := a.ipamHasOverlap(updated.Network, record.ID, updated.SectionID, updated.VRF, updated.ParentID); err != nil {
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
	record.VRF = updated.VRF
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
	if err := a.DB.Where("parent_id = ? AND section_id = ? AND vrf = ?", subnet.ID, subnet.SectionID, subnet.VRF).Find(&existing).Error; err != nil {
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

	csvReader := newCSVReader(reader)
	records, err := csvReader.ReadAll()
	if err != nil {
		c.HTML(http.StatusBadRequest, "ipam/address_import.html", gin.H{
			"Title":  "批量导入地址",
			"Subnet": subnet,
			"Error":  "CSV解析失败：" + err.Error(),
		})
		return
	}
	created := 0
	updated := 0
	skipped := 0
	for idx, row := range records {
		if len(row) == 0 {
			continue
		}
		ip := strings.TrimSpace(row[0])
		if idx == 0 && strings.EqualFold(ip, "ip") {
			continue
		}
		if ip == "" {
			skipped++
			continue
		}
		if _, err := netip.ParseAddr(ip); err != nil {
			skipped++
			continue
		}
		status := "used"
		unit := ""
		hostname := ""
		note := ""
		if len(row) > 1 {
			status = normalizeIPAMStatus(strings.TrimSpace(row[1]))
		}
		if len(row) > 2 {
			unit = strings.TrimSpace(row[2])
		}
		if len(row) > 3 {
			hostname = strings.TrimSpace(row[3])
		}
		if len(row) > 4 {
			note = strings.TrimSpace(row[4])
		}

		var existing models.IPAMAddress
		err := a.DB.Where("subnet_id = ? AND ip = ?", subnet.ID, ip).First(&existing).Error
		if err == nil {
			existing.Status = status
			existing.Unit = unit
			existing.Hostname = hostname
			existing.Note = note
			existing.UpdatedAt = time.Now()
			if err := a.DB.Save(&existing).Error; err == nil {
				updated++
			} else {
				skipped++
			}
			continue
		}
		record := models.IPAMAddress{
			SubnetID: subnet.ID,
			IP:       ip,
			Status:   status,
			Unit:     unit,
			Hostname: hostname,
			Note:     note,
		}
		if err := a.DB.Create(&record).Error; err == nil {
			created++
		} else {
			skipped++
		}
	}

	msg := fmt.Sprintf("导入完成：新增%d，更新%d，跳过%d", created, updated, skipped)
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

	writer := newCSVWriter(c.Writer)
	_ = writer.Write([]string{"ip", "status", "unit", "hostname", "note"})
	for _, addr := range addresses {
		_ = writer.Write([]string{addr.IP, addr.Status, addr.Unit, addr.Hostname, addr.Note})
	}
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

func (a *AppContext) ipamHasOverlap(cidr string, excludeID uint, sectionID uint, vrf string, parentID *uint) (bool, error) {
	query := a.DB.Model(&models.IPAMSubnet{}).Where("network && ?::cidr", cidr).Where("section_id = ?", sectionID).Where("vrf = ?", vrf)
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
		VRF:          strings.TrimSpace(c.PostForm("vrf")),
		RateMbps:     strings.TrimSpace(c.PostForm("rate_mbps")),
		VlanID:       strings.TrimSpace(c.PostForm("vlan_id")),
		L2Port:       strings.TrimSpace(c.PostForm("l2_port")),
		EgressDevice: strings.TrimSpace(c.PostForm("egress_device")),
		Description:  strings.TrimSpace(c.PostForm("description")),
	}
}

func bindSubnetForm(form ipamSubnetForm) (models.IPAMSubnet, error) {
	if form.SectionID == 0 {
		return models.IPAMSubnet{}, fmt.Errorf("请选择所属大区")
	}
	prefix, err := parseIPAMPrefix(form.Network)
	if err != nil {
		return models.IPAMSubnet{}, err
	}
	if form.Unit == "" {
		return models.IPAMSubnet{}, fmt.Errorf("使用单位不能为空")
	}
	if form.L2Port == "" {
		return models.IPAMSubnet{}, fmt.Errorf("二层节点端口不能为空")
	}
	if form.EgressDevice == "" {
		return models.IPAMSubnet{}, fmt.Errorf("出口设备名称不能为空")
	}
	if form.VRF == "" {
		return models.IPAMSubnet{}, fmt.Errorf("VRF 不能为空")
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
		VRF:          form.VRF,
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
		`SELECT subnet_id, COUNT(*) FROM ip_am_addresses WHERE subnet_id IN ? AND status <> 'free' GROUP BY subnet_id`,
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

func (a *AppContext) buildSectionItems(sections []models.IPAMSection, subnets []models.IPAMSubnet, used map[uint]int64) []ipamSectionItem {
	items := make([]ipamSectionItem, 0, len(sections))
	bySection := make(map[uint][]models.IPAMSubnet)
	for _, s := range subnets {
		bySection[s.SectionID] = append(bySection[s.SectionID], s)
	}
	for _, section := range sections {
		list := bySection[section.ID]
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
		maxFree := "未配置"
		maxHint := ""
		if section.RootCIDR != "" {
			if prefix, err := parseIPAMPrefix(section.RootCIDR); err == nil {
				freePrefix, vrf := findLargestFreePrefixByVRF(prefix, list)
				if freePrefix == "" {
					maxFree = "无空闲"
				} else {
					maxFree = freePrefix
				}
				if vrf != "" {
					maxHint = fmt.Sprintf("根网段：%s · VRF：%s", prefix.String(), vrf)
				} else {
					maxHint = prefix.String()
				}
			}
		}
		items = append(items, ipamSectionItem{
			ID:              section.ID,
			Name:            section.Name,
			RootCIDR:        section.RootCIDR,
			Description:     section.Description,
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

func buildSubnetTreeItems(subnets []models.IPAMSubnet, used map[uint]int64) []ipamSubnetItem {
	byParent := make(map[uint][]models.IPAMSubnet)
	var roots []models.IPAMSubnet
	for _, subnet := range subnets {
		if subnet.ParentID != nil {
			byParent[*subnet.ParentID] = append(byParent[*subnet.ParentID], subnet)
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
			items = append(items, ipamSubnetItem{
				ID:           subnet.ID,
				Network:      subnet.Network,
				Unit:         subnet.Unit,
				VRF:          subnet.VRF,
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
			})
			if children, ok := byParent[subnet.ID]; ok {
				walk(children, depth+1)
			}
		}
	}
	walk(roots, 0)
	return items
}

func (a *AppContext) searchIPAM(searchIP, searchUnit string) ([]ipamSearchSubnet, []ipamSearchAddress) {
	var subnetMatches []ipamSearchSubnet
	var addressMatches []ipamSearchAddress

	if searchIP != "" {
		addr, addrErr := netip.ParseAddr(searchIP)
		if addrErr == nil {
			var addrRows []struct {
				ID        uint
				IP        string
				Status    string
				Unit      string
				Hostname  string
				SubnetID  uint
				Network   string
				Section   string
				UpdatedAt time.Time
			}
			a.DB.Table("ip_am_addresses").
				Select("ip_am_addresses.id, ip_am_addresses.ip, ip_am_addresses.status, ip_am_addresses.unit, ip_am_addresses.hostname, ip_am_addresses.subnet_id, ip_am_subnets.network, ip_am_sections.name as section, ip_am_addresses.updated_at").
				Joins("left join ip_am_subnets on ip_am_subnets.id = ip_am_addresses.subnet_id").
				Joins("left join ip_am_sections on ip_am_sections.id = ip_am_subnets.section_id").
				Where("ip_am_addresses.ip = ?", addr.String()).Find(&addrRows)
			for _, row := range addrRows {
				addressMatches = append(addressMatches, ipamSearchAddress{
					ID:        row.ID,
					IP:        row.IP,
					Status:    ipamStatusLabel(row.Status),
					Unit:      row.Unit,
					Hostname:  row.Hostname,
					SubnetID:  row.SubnetID,
					Network:   row.Network,
					Section:   row.Section,
					UpdatedAt: row.UpdatedAt.Format("2006-01-02 15:04"),
				})
			}

			var subRows []struct {
				ID      uint
				Network string
				Unit    string
				VRF     string
				Section string
			}
			a.DB.Table("ip_am_subnets").
				Select("ip_am_subnets.id, ip_am_subnets.network, ip_am_subnets.unit, ip_am_subnets.vrf, ip_am_sections.name as section").
				Joins("left join ip_am_sections on ip_am_sections.id = ip_am_subnets.section_id").
				Where("?::inet <<= ip_am_subnets.network", addr.String()).Order("masklen(ip_am_subnets.network) desc").Find(&subRows)
			for _, row := range subRows {
				subnetMatches = append(subnetMatches, ipamSearchSubnet{
					ID:      row.ID,
					Network: row.Network,
					Unit:    row.Unit,
					VRF:     row.VRF,
					Section: row.Section,
				})
			}
		}
	}

	if searchUnit != "" {
		like := "%" + searchUnit + "%"
		var subRows []struct {
			ID      uint
			Network string
			Unit    string
			VRF     string
			Section string
		}
		a.DB.Table("ip_am_subnets").
			Select("ip_am_subnets.id, ip_am_subnets.network, ip_am_subnets.unit, ip_am_subnets.vrf, ip_am_sections.name as section").
			Joins("left join ip_am_sections on ip_am_sections.id = ip_am_subnets.section_id").
			Where("ip_am_subnets.unit ILIKE ?", like).Find(&subRows)
		for _, row := range subRows {
			subnetMatches = append(subnetMatches, ipamSearchSubnet{
				ID:      row.ID,
				Network: row.Network,
				Unit:    row.Unit,
				VRF:     row.VRF,
				Section: row.Section,
			})
		}

		var addrRows []struct {
			ID        uint
			IP        string
			Status    string
			Unit      string
			Hostname  string
			SubnetID  uint
			Network   string
			Section   string
			UpdatedAt time.Time
		}
		a.DB.Table("ip_am_addresses").
			Select("ip_am_addresses.id, ip_am_addresses.ip, ip_am_addresses.status, ip_am_addresses.unit, ip_am_addresses.hostname, ip_am_addresses.subnet_id, ip_am_subnets.network, ip_am_sections.name as section, ip_am_addresses.updated_at").
			Joins("left join ip_am_subnets on ip_am_subnets.id = ip_am_addresses.subnet_id").
			Joins("left join ip_am_sections on ip_am_sections.id = ip_am_subnets.section_id").
			Where("ip_am_addresses.unit ILIKE ?", like).Find(&addrRows)
		for _, row := range addrRows {
			addressMatches = append(addressMatches, ipamSearchAddress{
				ID:        row.ID,
				IP:        row.IP,
				Status:    ipamStatusLabel(row.Status),
				Unit:      row.Unit,
				Hostname:  row.Hostname,
				SubnetID:  row.SubnetID,
				Network:   row.Network,
				Section:   row.Section,
				UpdatedAt: row.UpdatedAt.Format("2006-01-02 15:04"),
			})
		}
	}

	return subnetMatches, addressMatches
}

func (a *AppContext) countUsedAddresses(subnetID uint) int64 {
	var count int64
	a.DB.Model(&models.IPAMAddress{}).Where("subnet_id = ? AND status <> 'free'", subnetID).Count(&count)
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

func findLargestFreePrefixByVRF(root netip.Prefix, subnets []models.IPAMSubnet) (string, string) {
	if len(subnets) == 0 {
		return "", ""
	}
	grouped := make(map[string][]models.IPAMSubnet)
	for _, subnet := range subnets {
		vrf := subnet.VRF
		grouped[vrf] = append(grouped[vrf], subnet)
	}
	var best string
	var bestVrf string
	bestBits := 129
	for vrf, list := range grouped {
		prefix := findLargestFreePrefix(root, list)
		if prefix == "" {
			continue
		}
		parsed, err := netip.ParsePrefix(prefix)
		if err != nil {
			continue
		}
		if parsed.Bits() < bestBits {
			bestBits = parsed.Bits()
			best = prefix
			bestVrf = vrf
		}
	}
	return best, bestVrf
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
		return nil
	}
	var parent models.IPAMSubnet
	if err := a.DB.First(&parent, *record.ParentID).Error; err != nil {
		return fmt.Errorf("父子网不存在")
	}
	if parent.SectionID != record.SectionID {
		return fmt.Errorf("父子网所属大区不一致")
	}
	if parent.VRF != record.VRF {
		return fmt.Errorf("父子网 VRF 不一致")
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
