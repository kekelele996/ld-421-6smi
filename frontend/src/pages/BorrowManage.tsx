import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  List,
  Modal,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  message
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, HistoryOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { createBorrow, fetchBorrows } from '../api/borrow'
import { fetchEquipment } from '../api/equipment'
import { useBorrowFlow } from '../hooks/useBorrowFlow'
import { StatusBadge } from '../components/common/StatusBadge'
import { StepIndicator } from '../components/common/StepIndicator'
import { usePagination } from '../hooks/usePagination'
import type { BorrowRecord, BorrowRenewal, CreateBorrowPayload, Equipment } from '../types'
import { useAuthStore } from '../stores/authStore'

const canApprove = (role?: string) => role === 'Admin' || role === 'LabManager'

const renewalTagColor: Record<string, string> = {
  Pending: 'gold',
  Approved: 'green',
  Rejected: 'red'
}
const renewalTagText: Record<string, string> = {
  Pending: '待审批',
  Approved: '已批准',
  Rejected: '已驳回'
}

export function BorrowManage() {
  const [items, setItems] = useState<BorrowRecord[]>([])
  const [equipment, setEquipment] = useState<Equipment[]>([])
  const [status, setStatus] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [renewTarget, setRenewTarget] = useState<BorrowRecord | null>(null)
  const [historyTarget, setHistoryTarget] = useState<BorrowRecord | null>(null)
  const [extendDays, setExtendDays] = useState(3)
  const [form] = Form.useForm<CreateBorrowPayload>()
  const pagination = usePagination()
  const user = useAuthStore((state) => state.user)
  const flow = useBorrowFlow()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const result = await fetchBorrows({
        page: pagination.page,
        page_size: pagination.pageSize,
        status: status || undefined
      })
      setItems(result.list)
      pagination.setTotal(result.total)
    } finally {
      setLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, pagination.page, pagination.pageSize])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    fetchEquipment({ page: 1, page_size: 100 }).then((res) => setEquipment(res.list))
  }, [])

  const submit = async () => {
    const values = await form.validateFields()
    await flow.submit({
      equipmentId: values.equipmentId,
      borrowDate: dayjs(values.borrowDate).format('YYYY-MM-DD'),
      expectedReturnDate: dayjs(values.expectedReturnDate).format('YYYY-MM-DD'),
      reason: values.reason
    })
    message.success('借用申请已提交')
    setModalOpen(false)
    form.resetFields()
    load()
  }

  const approve = async (id: number) => {
    await flow.approve(id)
    message.success('已审批通过')
    load()
  }

  const reject = async (id: number) => {
    await flow.reject(id)
    message.success('已驳回')
    load()
  }

  const confirmReturn = (id: number) => {
    let condition = 'Good'
    Modal.confirm({
      title: '确认归还',
      content: (
        <Select
          style={{ width: '100%', marginTop: 12 }}
          defaultValue="Good"
          onChange={(value) => (condition = value)}
          options={[
            { label: '良好', value: 'Good' },
            { label: '损坏', value: 'Damaged' },
            { label: '丢失', value: 'Lost' }
          ]}
        />
      ),
      onOk: async () => {
        await flow.confirmReturn(id, {
          actualReturnDate: dayjs().format('YYYY-MM-DD'),
          returnCondition: condition
        })
        message.success('归还已确认')
        load()
      }
    })
  }

  const submitRenewal = async () => {
    if (!renewTarget) return
    await flow.requestRenewal(renewTarget.id, extendDays)
    message.success(`续借申请已提交，延长 ${extendDays} 天`)
    setRenewTarget(null)
    setExtendDays(3)
    load()
  }

  const approveRenew = async (renewal: BorrowRenewal) => {
    await flow.approvePendingRenewal(renewal.id)
    message.success('续借已批准，归还日期已更新')
    load()
  }

  const rejectRenew = (renewal: BorrowRenewal) => {
    let reason = ''
    Modal.confirm({
      title: '驳回续借申请',
      content: (
        <Input.TextArea
          style={{ marginTop: 12 }}
          rows={3}
          placeholder="驳回原因（选填），驳回不改变原归还日期"
          onChange={(e) => (reason = e.target.value)}
        />
      ),
      onOk: async () => {
        await flow.rejectPendingRenewal(renewal.id, reason || undefined)
        message.success('续借申请已驳回')
        load()
      }
    })
  }

  const columns: ColumnsType<BorrowRecord> = [
    { title: '设备', dataIndex: 'equipmentName', width: 160 },
    { title: '借用人', dataIndex: 'borrowerName', width: 100 },
    { title: '借用日期', dataIndex: 'borrowDate', width: 110, render: (v: string) => v?.slice(0, 10) },
    {
      title: '预计归还',
      dataIndex: 'expectedReturnDate',
      width: 130,
      render: (v: string, record) => (
        <Space direction="vertical" size={0}>
          <span>{v?.slice(0, 10)}</span>
          {record.originalDueDate && (
            <span style={{ fontSize: 12, color: '#999' }}>原到期 {record.originalDueDate.slice(0, 10)}</span>
          )}
        </Space>
      )
    },
    {
      title: '续借',
      width: 130,
      render: (_, record) => {
        if (record.pendingRenewal) {
          return <Tag color="gold">续借审批中 +{record.pendingRenewal.extendDays}天</Tag>
        }
        const approvedCount = (record.renewals || []).filter((r) => r.status === 'Approved').length
        return approvedCount > 0 ? <Tag color="green">已续借 {approvedCount} 次</Tag> : <span style={{ color: '#bbb' }}>—</span>
      }
    },
    { title: '事由', dataIndex: 'reason', ellipsis: true },
    { title: '状态', dataIndex: 'status', width: 100, render: (v: string) => <StatusBadge status={v} /> },
    { title: '审批人', dataIndex: 'approverName', width: 100 },
    {
      title: '流程',
      width: 210,
      render: (_, record) => <StepIndicator current={flow.stepOf(record.status)} />
    },
    {
      title: '操作',
      width: 260,
      render: (_, record) => (
        <Space size={4} wrap>
          {record.status === 'Pending' && canApprove(user?.roleCode) ? (
            <>
              <Button size="small" type="primary" onClick={() => approve(record.id)}>
                通过
              </Button>
              <Button size="small" danger onClick={() => reject(record.id)}>
                驳回
              </Button>
            </>
          ) : null}
          {record.pendingRenewal && canApprove(user?.roleCode) ? (
            <>
              <Button size="small" type="primary" onClick={() => approveRenew(record.pendingRenewal!)}>
                批准续借
              </Button>
              <Button size="small" danger onClick={() => rejectRenew(record.pendingRenewal!)}>
                驳回续借
              </Button>
            </>
          ) : null}
          {flow.canApplyRenewal(record, user?.id) ? (
            <Button size="small" type="link" onClick={() => setRenewTarget(record)}>
              申请续借
            </Button>
          ) : null}
          {(record.status === 'Approved' || record.status === 'Overdue') && canApprove(user?.roleCode) ? (
            <Button size="small" type="link" onClick={() => confirmReturn(record.id)}>
              确认归还
            </Button>
          ) : null}
          {(record.renewals?.length ?? 0) > 0 ? (
            <Button size="small" type="link" icon={<HistoryOutlined />} onClick={() => setHistoryTarget(record)}>
              续借记录
            </Button>
          ) : null}
        </Space>
      )
    }
  ]

  return (
    <Card
      title="借用管理"
      extra={
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
          提交借用
        </Button>
      }
    >
      {items.some((item) => item.status === 'Overdue') ? (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 12 }}
          message="存在逾期未归还的借用，逾期借用不可申请续借，请尽快跟进归还"
        />
      ) : null}
      <Tabs
        activeKey={status}
        onChange={(key) => {
          setStatus(key)
          pagination.setPage(1)
        }}
        items={[
          { key: '', label: '全部' },
          { key: 'Pending', label: '待审批' },
          { key: 'Approved', label: '已通过' },
          { key: 'Rejected', label: '已驳回' },
          { key: 'Returned', label: '已归还' },
          { key: 'Overdue', label: '已逾期' }
        ]}
      />
      <Table
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={items}
        pagination={{
          current: pagination.page,
          pageSize: pagination.pageSize,
          total: pagination.total,
          showSizeChanger: true,
          onChange: (page, pageSize) => {
            pagination.setPage(page)
            pagination.setPageSize(pageSize)
          }
        }}
      />

      <Modal title="提交借用申请" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={submit}>
        <Form form={form} layout="vertical">
          <Form.Item name="equipmentId" label="选择设备" rules={[{ required: true }]}>
            <Select
              placeholder="选择可用设备"
              options={equipment
                .filter((item) => item.status === 'Available')
                .map((item) => ({ label: `${item.name}（${item.code}）`, value: item.id }))}
            />
          </Form.Item>
          <Form.Item name="borrowDate" label="借用日期" rules={[{ required: true }]}>
            <Input placeholder="YYYY-MM-DD" />
          </Form.Item>
          <Form.Item name="expectedReturnDate" label="预计归还日期" rules={[{ required: true }]}>
            <Input placeholder="YYYY-MM-DD" />
          </Form.Item>
          <Form.Item name="reason" label="借用事由">
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="申请续借"
        open={!!renewTarget}
        onCancel={() => setRenewTarget(null)}
        onOk={submitRenewal}
        okText="提交申请"
        destroyOnClose
      >
        {renewTarget ? (
          <div>
            <p>
              设备：<strong>{renewTarget.equipmentName}</strong>
            </p>
            <p>
              当前预计归还日：{renewTarget.expectedReturnDate.slice(0, 10)}
              {renewTarget.originalDueDate ? (
                <span style={{ color: '#999' }}>（原到期 {renewTarget.originalDueDate.slice(0, 10)}）</span>
              ) : null}
            </p>
            <Form layout="vertical">
              <Form.Item label="延长天数（1-7 天，须在预计归还日前申请）" required>
                <InputNumber min={1} max={7} value={extendDays} onChange={(v) => setExtendDays(v ?? 1)} />
              </Form.Item>
            </Form>
            <Alert type="info" showIcon message={`续借后预计归还日：${dayjs(renewTarget.expectedReturnDate).add(extendDays, 'day').format('YYYY-MM-DD')}`} />
          </div>
        ) : null}
      </Modal>

      <Modal title="续借记录" open={!!historyTarget} footer={null} onCancel={() => setHistoryTarget(null)}>
        {historyTarget ? (
          <List
            dataSource={[...(historyTarget.renewals || [])].reverse()}
            renderItem={(renewal) => (
              <List.Item>
                <List.Item.Meta
                  title={
                    <Space>
                      <Tag color={renewalTagColor[renewal.status]}>{renewalTagText[renewal.status] || renewal.status}</Tag>
                      <span>延长 {renewal.extendDays} 天</span>
                    </Space>
                  }
                  description={
                    <Space direction="vertical" size={0}>
                      <span>申请时间：{renewal.createdAt.slice(0, 10)}</span>
                      <span>续借后归还日：{renewal.newDueDate.slice(0, 10)}</span>
                      {renewal.reviewerName ? <span>审批人：{renewal.reviewerName}</span> : null}
                      {renewal.reviewReason ? <span>审批意见：{renewal.reviewReason}</span> : null}
                    </Space>
                  }
                />
              </List.Item>
            )}
          />
        ) : null}
      </Modal>
    </Card>
  )
}

export default BorrowManage
