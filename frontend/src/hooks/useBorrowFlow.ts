import { useState } from 'react'
import dayjs from 'dayjs'
import {
  applyRenewal,
  approveBorrow,
  approveRenewal,
  createBorrow,
  rejectBorrow,
  rejectRenewal,
  returnBorrow
} from '../api/borrow'
import { BorrowStatus } from '../types/enums'
import type { BorrowRecord, CreateBorrowPayload, ReturnBorrowPayload } from '../types'

const STATUS_STEP: Record<string, number> = {
  Pending: 0,
  Approved: 1,
  Returned: 3,
  Rejected: 1,
  Overdue: 2
}

export function useBorrowFlow() {
  const [loading, setLoading] = useState(false)

  const submit = async (payload: CreateBorrowPayload) => {
    setLoading(true)
    try {
      const result = await createBorrow(payload)
      return result
    } finally {
      setLoading(false)
    }
  }

  const approve = async (id: number | string) => {
    setLoading(true)
    try {
      await approveBorrow(id)
    } finally {
      setLoading(false)
    }
  }

  const reject = async (id: number | string) => {
    setLoading(true)
    try {
      await rejectBorrow(id)
    } finally {
      setLoading(false)
    }
  }

  const confirmReturn = async (id: number | string, payload: ReturnBorrowPayload) => {
    setLoading(true)
    try {
      await returnBorrow(id, payload)
    } finally {
      setLoading(false)
    }
  }

  const requestRenewal = async (id: number | string, extendDays: number) => {
    setLoading(true)
    try {
      return await applyRenewal(id, { extendDays })
    } finally {
      setLoading(false)
    }
  }

  const approvePendingRenewal = async (renewalId: number | string) => {
    setLoading(true)
    try {
      await approveRenewal(renewalId)
    } finally {
      setLoading(false)
    }
  }

  const rejectPendingRenewal = async (renewalId: number | string, reason?: string) => {
    setLoading(true)
    try {
      await rejectRenewal(renewalId, reason)
    } finally {
      setLoading(false)
    }
  }

  const stepOf = (status: string) => STATUS_STEP[status] ?? 0

  // 续借入口：仅已审批、未逾期、无待审批续借、且仍在预计归还日之前开放；归还后关闭。
  const canApplyRenewal = (record: BorrowRecord, currentUserId?: number) => {
    if (record.status !== BorrowStatus.Approved) return false
    if (currentUserId !== undefined && record.borrowerId !== currentUserId) return false
    if (record.pendingRenewal) return false
    return dayjs(record.expectedReturnDate).startOf('day').isAfter(dayjs().startOf('day'))
  }

  return {
    loading,
    submit,
    approve,
    reject,
    confirmReturn,
    requestRenewal,
    approvePendingRenewal,
    rejectPendingRenewal,
    stepOf,
    canApplyRenewal
  }
}

export default useBorrowFlow
