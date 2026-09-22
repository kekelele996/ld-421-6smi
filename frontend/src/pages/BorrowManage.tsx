import { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  message
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { createBorrow, fetchBorrows } from '../api/borrow'
import { fetchEquipment } from '../api/equipment'
import { useBorrowFlow } from '../hooks/useBorrowFlow'
import { StatusBadge } from '../components/common/StatusBadge'
import { StepIndicator } from '../components/common/StepIndicator'
import { usePagination } from '../hooks/usePagination'
import type { BorrowRecord, CreateBorrowPayload, CreateRenewalPayload, Equipment } from '../types'
import { useAuthStore } from '../stores/authStore'

const canApprove = (role?: string) => role === 'Admin' || role === 'LabManager'

const formatDate = (value?: string) => value?.slice(0, 10) ?? '-'

export function BorrowManage() {
  const [items, setItems] = useState<BorrowRecord[]>([])
  const [equipment, setEquipment] = useState<Equipment[]>([])
  const [status, setStatus] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [renewTarget, setRenewTarget] = useState<BorrowRecord | null>(null)
  const [rejectRenewTarget, setRejectRenewTarget] = useState<BorrowRecord | null>(null)
  const [rejectComment, setRejectComment] = useState('')
  const [form] = Form.useForm<CreateBorrowPayload>()
  const [renewForm] = Form.useForm<CreateRenewalPayload>()
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

  const openRenew = (record: BorrowRecord) => {
    setRenewTarget(record)
    renewForm.setFieldsValue({ extendDays: 3, reason: undefined })
  }

  const submitRenew = async () => {
    if (!renewTarget) return
    const values = await renewForm.validateFields()
    await flow.applyRenew(renewTarget.id, values)
    message.success('续借申请已提交，等待审批')
    setRenewTarget(null)
    load()
  }

  const approveRenew = async (record: BorrowRecord) => {
    if (!record.pendingRenewal) return
    await flow.approveRenew(record.pendingRenewal.id)
    message.success('续借已批准，归还日期已更新')
    load()
  }

  const submitRejectRenew = async () => {
    if (!rejectRenewTarget?.pendingRenewal) return
    await flow.rejectRenew(rejectRenewTarget.pendingRenewal.id, rejectComment || undefined)
    message.success('续借申请已驳回')
    setRejectRenewTarget(null)
    setRejectComment('')
    load()
  }

  // 借用人本人且在借用期内、无待审批续借、尚未到归还日，才允许申请续借。
  const canApplyRenew = (record: BorrowRecord) =>
    record.status === 'Approved' &&
    !record.pendingRenewal &&
    record.borrowerId === user?.id &&
    dayjs().isBefore(dayjs(record.expectedReturnDate), 'day')

  const columns: ColumnsType<BorrowRecord> = [
    { title: '设备', dataIndex: 'equipmentName', width: 150 },
    { title: '借用人', dataIndex: 'borrowerName', width: 90 },
    { title: '借用日期', dataIndex: 'borrowDate', width: 110, render: (v: string) => formatDate(v) },
    {
      title: '预计归还',
      dataIndex: 'expectedReturnDate',
      width: 130,
      render: (v: string, record) => {
        const extended = record.originalExpectedReturn && record.originalExpectedReturn.slice(0, 10) !== v.slice(0, 10)
        return (
          <Space direction="vertical" size={0}>
            <span style={record.status === 'Overdue' ? { color: '#ff4d4f' } : undefined}>{formatDate(v)}</span>
            {extended ? (
              <Tooltip title="续借前的原预计归还日期">
                <span style={{ fontSize: 12, color: '#999' }}>
                  原 <s>{formatDate(record.originalExpectedReturn)}</s>
                </span>
              </Tooltip>
            ) : null}
          </Space>
        )
      }
    },
    { title: '事由', dataIndex: 'reason', ellipsis: true },
    {
      title: '状态',
      dataIndex: 'status',
      width: 110,
      render: (v: string) => <StatusBadge status={v} />
    },
    {
      title: '续借',
      width: 130,
      render: (_, record) =>
        record.pendingRenewal ? (
          <Tooltip
            title={`申请延长 ${record.pendingRenewal.extendDays} 天，期望 ${formatDate(
              record.pendingRenewal.requestedDueDate
            )} 归还`}
          >
            <Tag color="processing">续借审批中 +{record.pendingRenewal.extendDays}天</Tag>
          </Tooltip>
        ) : (
          <span style={{ color: '#bbb' }}>-</span>
        )
    },
    { title: '审批人', dataIndex: 'approverName', width: 90 },
    {
      title: '流程',
      width: 200,
      render: (_, record) => <StepIndicator current={flow.stepOf(record.status)} />
    },
    {
      title: '操作',
      width: 230,
      fixed: 'right',
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
          {(record.status === 'Approved' || record.status === 'Overdue') && canApprove(user?.roleCode) ? (
            <Button size="small" type="link" onClick={() => confirmReturn(record.id)}>
              确认归还
            </Button>
          ) : null}
          {canApplyRenew(record) ? (
            <Button size="small" type="link" onClick={() => openRenew(record)}>
              申请续借
            </Button>
          ) : null}
          {record.pendingRenewal && canApprove(user?.roleCode) ? (
            <>
              <Button size="small" type="link" onClick={() => approveRenew(record)}>
                批准续借
              </Button>
              <Button size="small" type="link" danger onClick={() => setRejectRenewTarget(record)}>
                驳回续借
              </Button>
            </>
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
        loading={loading || flow.loading}
        columns={columns}
        dataSource={items}
        scroll={{ x: 1400 }}
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
        open={renewTarget !== null}
        onCancel={() => setRenewTarget(null)}
        onOk={submitRenew}
        okText="提交申请"
        destroyOnClose
      >
        {renewTarget ? (
          <div style={{ marginBottom: 12, color: '#666' }}>
            <div>设备：{renewTarget.equipmentName}</div>
            <div>当前预计归还：{formatDate(renewTarget.expectedReturnDate)}</div>
          </div>
        ) : null}
        <Form form={renewForm} layout="vertical" initialValues={{ extendDays: 3 }}>
          <Form.Item
            name="extendDays"
            label="延长天数（1-7 天）"
            rules={[{ required: true, message: '请选择延长天数' }]}
          >
            <InputNumber min={1} max={7} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item shouldUpdate noStyle>
            {() => {
              const days = renewForm.getFieldValue('extendDays') ?? 0
              const next = renewTarget
                ? dayjs(renewTarget.expectedReturnDate).add(days, 'day').format('YYYY-MM-DD')
                : '-'
              return (
                <div style={{ marginBottom: 12 }}>
                  续借后预计归还：<b>{next}</b>
                </div>
              )
            }}
          </Form.Item>
          <Form.Item name="reason" label="续借事由">
            <Input.TextArea rows={3} maxLength={512} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="驳回续借申请"
        open={rejectRenewTarget !== null}
        onCancel={() => {
          setRejectRenewTarget(null)
          setRejectComment('')
        }}
        onOk={submitRejectRenew}
        okText="确认驳回"
        okButtonProps={{ danger: true }}
      >
        <p>驳回后原借用归还日期不变，借用人可重新提交申请。</p>
        <Input.TextArea
          rows={3}
          maxLength={512}
          placeholder="审批意见（选填）"
          value={rejectComment}
          onChange={(e) => setRejectComment(e.target.value)}
        />
      </Modal>
    </Card>
  )
}

export default BorrowManage
