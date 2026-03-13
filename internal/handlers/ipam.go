package handlers

import (
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"duty-log-system/internal/middleware"
	"duty-log-system/internal/models"

	"github.com/gin-gonic/gin"
)

type ipamListItem struct {
	ID           uint
	Network      string
	IPVersion    string
	PrefixLength int
	Unit         string
	RateMbps     int
	VlanID       int
	L2Port       string
	EgressDevice string
	UpdatedAt    string
}

type ipamFormView struct {
	ID           uint
	Network      string
	Unit         string
	RateMbps     string
	VlanID       string
	L2Port       string
	EgressDevice string
}

func registerIPAMRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/ipam", app.ipamList)
	group.GET("/ipam/create", app.ipamCreatePage)
	group.POST("/ipam/create", app.ipamCreate)
	group.GET("/ipam/:id", app.ipamDetail)
	group.GET("/ipam/:id/edit", app.ipamEditPage)
	group.POST("/ipam/:id/edit", app.ipamUpdate)
	group.POST("/ipam/:id/delete", app.ipamDelete)
}

func (a *AppContext) ipamList(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	searchIP := strings.TrimSpace(c.Query("ip"))
	searchUnit := strings.TrimSpace(c.Query("unit"))
	searchKeyword := strings.TrimSpace(c.Query("keyword"))

	query := a.DB.Order("network")

	var parsedIP netip.Addr
	if searchIP != "" {
		if addr, err := netip.ParseAddr(searchIP); err == nil {
			parsedIP = addr
		} else if prefix, perr := netip.ParsePrefix(searchIP); perr == nil {
			parsedIP = prefix.Addr()
		} else {
			c.HTML(http.StatusBadRequest, "ipam/list.html", gin.H{
				"Title":       "IPAM 资产管理",
				"Items":       []ipamListItem{},
				"CanManage":   currentUser.IsAdmin,
				"Error":       "IP 格式错误，请输入 IPv4/IPv6 地址",
				"SearchIP":    searchIP,
				"SearchUnit":  searchUnit,
				"SearchKey":   searchKeyword,
				"SearchCount": 0,
			})
			return
		}
		query = query.Where("?::inet <<= network", parsedIP.String()).Order("masklen(network) desc")
	}
	if searchUnit != "" {
		like := "%" + searchUnit + "%"
		query = query.Where("unit ILIKE ?", like)
	}
	if searchKeyword != "" {
		like := "%" + searchKeyword + "%"
		query = query.Where("unit ILIKE ? OR l2_port ILIKE ? OR egress_device ILIKE ?", like, like, like)
	}

	var records []models.IPAMSubnet
	if err := query.Find(&records).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "coming_soon.html", gin.H{
			"Title":   "IPAM 资产管理",
			"Path":    "/ipam",
			"Message": "读取 IPAM 记录失败：" + err.Error(),
		})
		return
	}

	items := make([]ipamListItem, 0, len(records))
	for _, record := range records {
		version := "未知"
		prefixLen := 0
		if prefix, err := netip.ParsePrefix(record.Network); err == nil {
			if prefix.Addr().Is4() {
				version = "IPv4"
			} else if prefix.Addr().Is6() {
				version = "IPv6"
			}
			prefixLen = prefix.Bits()
		}
		items = append(items, ipamListItem{
			ID:           record.ID,
			Network:      record.Network,
			IPVersion:    version,
			PrefixLength: prefixLen,
			Unit:         record.Unit,
			RateMbps:     record.RateMbps,
			VlanID:       record.VlanID,
			L2Port:       record.L2Port,
			EgressDevice: record.EgressDevice,
			UpdatedAt:    record.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}

	c.HTML(http.StatusOK, "ipam/list.html", gin.H{
		"Title":       "IPAM 资产管理",
		"Items":       items,
		"CanManage":   currentUser.IsAdmin,
		"Msg":         strings.TrimSpace(c.Query("msg")),
		"Error":       strings.TrimSpace(c.Query("error")),
		"SearchIP":    searchIP,
		"SearchUnit":  searchUnit,
		"SearchKey":   searchKeyword,
		"SearchCount": len(items),
	})
}

func (a *AppContext) ipamCreatePage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可创建 IPAM 记录")
		return
	}
	a.renderIPAMForm(c, http.StatusOK, "新建 IPAM 资产", "/ipam/create", ipamFormView{}, "")
}

func (a *AppContext) ipamCreate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可创建 IPAM 记录")
		return
	}

	record, form, err := bindIPAMForm(c)
	if err != nil {
		a.renderIPAMForm(c, http.StatusBadRequest, "新建 IPAM 资产", "/ipam/create", form, err.Error())
		return
	}

	if overlap, err := a.ipamHasOverlap(record.Network, 0); err != nil {
		a.renderIPAMForm(c, http.StatusBadRequest, "新建 IPAM 资产", "/ipam/create", form, "检查重叠失败："+err.Error())
		return
	} else if overlap {
		a.renderIPAMForm(c, http.StatusBadRequest, "新建 IPAM 资产", "/ipam/create", form, "地址段与已有记录重叠，请检查 CIDR")
		return
	}

	if err := a.DB.Create(&record).Error; err != nil {
		a.renderIPAMForm(c, http.StatusBadRequest, "新建 IPAM 资产", "/ipam/create", form, ipamWriteErrorMessage("创建失败：", err))
		return
	}
	c.Redirect(http.StatusFound, "/ipam?msg=创建成功")
}

func (a *AppContext) ipamDetail(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效记录ID")
		return
	}

	var record models.IPAMSubnet
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=记录不存在")
		return
	}

	version := "未知"
	prefixLen := 0
	if prefix, err := netip.ParsePrefix(record.Network); err == nil {
		if prefix.Addr().Is4() {
			version = "IPv4"
		} else if prefix.Addr().Is6() {
			version = "IPv6"
		}
		prefixLen = prefix.Bits()
	}

	c.HTML(http.StatusOK, "ipam/detail.html", gin.H{
		"Title":       "IPAM 资产预览",
		"Record":      record,
		"IPVersion":   version,
		"PrefixLen":   prefixLen,
		"CanManage":   currentUser.IsAdmin,
		"UpdatedTime": record.UpdatedAt.Format("2006-01-02 15:04"),
	})
}

func (a *AppContext) ipamEditPage(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可编辑 IPAM 记录")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效记录ID")
		return
	}

	var record models.IPAMSubnet
	if err := a.DB.First(&record, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=记录不存在")
		return
	}

	form := ipamFormView{
		ID:           record.ID,
		Network:      record.Network,
		Unit:         record.Unit,
		RateMbps:     strconv.Itoa(record.RateMbps),
		VlanID:       strconv.Itoa(record.VlanID),
		L2Port:       record.L2Port,
		EgressDevice: record.EgressDevice,
	}

	a.renderIPAMForm(c, http.StatusOK, "编辑 IPAM 资产", "/ipam/"+strconv.FormatUint(id, 10)+"/edit", form, "")
}

func (a *AppContext) ipamUpdate(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可编辑 IPAM 记录")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效记录ID")
		return
	}

	var existing models.IPAMSubnet
	if err := a.DB.First(&existing, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error=记录不存在")
		return
	}

	record, form, bindErr := bindIPAMForm(c)
	form.ID = existing.ID
	if bindErr != nil {
		a.renderIPAMForm(c, http.StatusBadRequest, "编辑 IPAM 资产", "/ipam/"+strconv.FormatUint(id, 10)+"/edit", form, bindErr.Error())
		return
	}

	if overlap, err := a.ipamHasOverlap(record.Network, existing.ID); err != nil {
		a.renderIPAMForm(c, http.StatusBadRequest, "编辑 IPAM 资产", "/ipam/"+strconv.FormatUint(id, 10)+"/edit", form, "检查重叠失败："+err.Error())
		return
	} else if overlap {
		a.renderIPAMForm(c, http.StatusBadRequest, "编辑 IPAM 资产", "/ipam/"+strconv.FormatUint(id, 10)+"/edit", form, "地址段与已有记录重叠，请检查 CIDR")
		return
	}

	existing.Network = record.Network
	existing.Unit = record.Unit
	existing.RateMbps = record.RateMbps
	existing.VlanID = record.VlanID
	existing.L2Port = record.L2Port
	existing.EgressDevice = record.EgressDevice
	existing.UpdatedAt = time.Now()

	if err := a.DB.Save(&existing).Error; err != nil {
		a.renderIPAMForm(c, http.StatusBadRequest, "编辑 IPAM 资产", "/ipam/"+strconv.FormatUint(id, 10)+"/edit", form, ipamWriteErrorMessage("更新失败：", err))
		return
	}
	c.Redirect(http.StatusFound, "/ipam?msg=更新成功")
}

func (a *AppContext) ipamDelete(c *gin.Context) {
	currentUser, err := middleware.CurrentUser(c, a.DB)
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	if !currentUser.IsAdmin {
		c.Redirect(http.StatusFound, "/ipam?error=仅管理员可删除 IPAM 记录")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/ipam?error=无效记录ID")
		return
	}

	if err := a.DB.Delete(&models.IPAMSubnet{}, uint(id)).Error; err != nil {
		c.Redirect(http.StatusFound, "/ipam?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/ipam?msg=删除成功")
}

func (a *AppContext) ipamHasOverlap(cidr string, excludeID uint) (bool, error) {
	query := a.DB.Model(&models.IPAMSubnet{}).Where("network && ?::cidr", cidr)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func bindIPAMForm(c *gin.Context) (models.IPAMSubnet, ipamFormView, error) {
	form := ipamFormView{
		Network:      strings.TrimSpace(c.PostForm("network")),
		Unit:         strings.TrimSpace(c.PostForm("unit")),
		RateMbps:     strings.TrimSpace(c.PostForm("rate_mbps")),
		VlanID:       strings.TrimSpace(c.PostForm("vlan_id")),
		L2Port:       strings.TrimSpace(c.PostForm("l2_port")),
		EgressDevice: strings.TrimSpace(c.PostForm("egress_device")),
	}

	prefix, err := parseIPAMPrefix(form.Network)
	if err != nil {
		return models.IPAMSubnet{}, form, err
	}
	if form.Unit == "" {
		return models.IPAMSubnet{}, form, fmt.Errorf("使用单位不能为空")
	}
	rate, err := parsePositiveInt(form.RateMbps, "速率(Mbps)", 1, 1000000)
	if err != nil {
		return models.IPAMSubnet{}, form, err
	}
	vlan, err := parsePositiveInt(form.VlanID, "VLAN ID", 1, 4094)
	if err != nil {
		return models.IPAMSubnet{}, form, err
	}
	if form.L2Port == "" {
		return models.IPAMSubnet{}, form, fmt.Errorf("二层节点端口不能为空")
	}
	if form.EgressDevice == "" {
		return models.IPAMSubnet{}, form, fmt.Errorf("出口设备名称不能为空")
	}

	record := models.IPAMSubnet{
		Network:      prefix.String(),
		Unit:         form.Unit,
		RateMbps:     rate,
		VlanID:       vlan,
		L2Port:       form.L2Port,
		EgressDevice: form.EgressDevice,
	}
	return record, form, nil
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

func (a *AppContext) renderIPAMForm(c *gin.Context, statusCode int, title, action string, form ipamFormView, errMsg string) {
	c.HTML(statusCode, "ipam/form.html", gin.H{
		"Title":  title,
		"Action": action,
		"Form":   form,
		"Error":  errMsg,
	})
}
