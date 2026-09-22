import { useEffect, useState } from 'react'
import { Card, Col, List, Row, Skeleton } from 'antd'
import { DashboardOutlined, SwapOutlined, CalendarOutlined, ClockCircleOutlined, RedoOutlined } from '@ant-design/icons'
import ReactECharts from 'echarts-for-react'
import { fetchDashboardStats, type DashboardStats } from '../api/dashboard'
import { AlertBanner } from '../components/common/AlertBanner'
import { StatCard } from '../components/common/StatCard'
import { StatusBadge } from '../components/common/StatusBadge'

export function Dashboard() {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchDashboardStats()
      .then(setStats)
      .finally(() => setLoading(false))
  }, [])

  const totalEquipment = stats
    ? Object.values(stats.statusDistribution).reduce((sum, value) => sum + value, 0)
    : 0
  const statusPieOption = {
    tooltip: { trigger: 'item' },
    legend: { bottom: 0 },
    series: [
      {
        name: '设备状态',
        type: 'pie',
        radius: ['38%', '65%'],
        data: Object.entries(stats?.statusDistribution || {}).map(([name, value]) => ({ name, value }))
      }
    ]
  }
  const topBorrowsOption = {
    tooltip: {},
    grid: { left: 40, right: 16, top: 10, bottom: 30 },
    xAxis: { type: 'category', data: (stats?.topBorrows || []).map((item) => item.name) },
    yAxis: { type: 'value' },
    series: [{ type: 'bar', data: (stats?.topBorrows || []).map((item) => item.count), itemStyle: { color: '#1677ff' } }]
  }

  if (loading) return <Skeleton active />

  return (
    <div>
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title="设备总数" value={totalEquipment} icon={<DashboardOutlined />} color="#1677ff" />
        </Col>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title="待审批借用" value={stats?.pendingBorrows ?? 0} icon={<SwapOutlined />} color="#fa8c16" />
        </Col>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title="逾期借用" value={stats?.overdueBorrows ?? 0} icon={<ClockCircleOutlined />} color="#f5222d" />
        </Col>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title="待审批续借" value={stats?.pendingRenewals ?? 0} icon={<RedoOutlined />} color="#13c2c2" />
        </Col>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title="待审批预约" value={stats?.pendingReservations ?? 0} icon={<CalendarOutlined />} color="#722ed1" />
        </Col>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title="即将过保设备" value={stats?.expiringWarranty.length ?? 0} color="#eb2f96" />
        </Col>
      </Row>

      {(stats?.overdueBorrows ?? 0) > 0 ? (
        <div style={{ marginTop: 16 }}>
          <AlertBanner
            type="error"
            message={`当前有 ${stats?.overdueBorrows ?? 0} 笔借用已逾期未归还`}
            description="逾期借用会在借用列表中同步标记为“已逾期”，且不可再申请续借，请尽快跟进归还。"
          />
        </div>
      ) : null}

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={12}>
          <Card title="设备状态分布" size="small">
            <ReactECharts option={statusPieOption} style={{ height: 280 }} />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="本月借用次数 TOP10" size="small">
            {(stats?.topBorrows.length ?? 0) > 0 ? (
              <ReactECharts option={topBorrowsOption} style={{ height: 280 }} />
            ) : (
              <div style={{ height: 280, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#999' }}>
                暂无借用记录
              </div>
            )}
          </Card>
        </Col>
      </Row>

      <Card title="即将过保设备" size="small" style={{ marginTop: 16 }}>
        {(stats?.expiringWarranty.length ?? 0) > 0 ? (
          <List
            dataSource={stats?.expiringWarranty}
            renderItem={(item) => (
              <List.Item>
                <AlertBanner
                  type="warning"
                  message={`${item.name}（${item.code}）`}
                  description={`保修到期日：${item.warrantyExpiry ? item.warrantyExpiry.slice(0, 10) : '未知'}`}
                />
              </List.Item>
            )}
          />
        ) : (
          <AlertBanner type="success" message="当前没有即将过保的设备" />
        )}
      </Card>

      <Card title="借用状态快速预览" size="small" style={{ marginTop: 16 }}>
        <List
          dataSource={[
            { name: '待审批借用', status: 'Pending', count: stats?.pendingBorrows ?? 0 },
            { name: '已逾期借用', status: 'Overdue', count: stats?.overdueBorrows ?? 0 },
            { name: '待审批续借', status: 'RenewalPending', count: stats?.pendingRenewals ?? 0 }
          ]}
          renderItem={(item) => (
            <List.Item actions={[<span key="count" style={{ fontWeight: 600 }}>{item.count}</span>, <StatusBadge key={item.status} status={item.status} />]}>
              <span>{item.name}</span>
            </List.Item>
          )}
        />
      </Card>
    </div>
  )
}

export default Dashboard
