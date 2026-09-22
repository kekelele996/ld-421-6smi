import type { RenewalStatus } from './enums'

export interface BorrowRenewal {
  id: number
  borrowId: number
  extendDays: number
  newDueDate: string
  status: RenewalStatus | string
  reviewerId?: number
  reviewerName?: string
  reviewReason?: string
  createdAt: string
}

export interface BorrowRecord {
  id: number
  equipmentId: number
  equipmentName?: string
  equipmentCode?: string
  borrowerId: number
  borrowerName?: string
  borrowDate: string
  expectedReturnDate: string
  originalDueDate?: string
  actualReturnDate?: string
  reason: string
  status: string
  approverId?: number
  approverName?: string
  returnCondition?: string
  pendingRenewal?: BorrowRenewal
  renewals?: BorrowRenewal[]
  createdAt: string
}

export interface CreateBorrowPayload {
  equipmentId: number
  borrowDate: string
  expectedReturnDate: string
  reason?: string
}

export interface ReturnBorrowPayload {
  actualReturnDate: string
  returnCondition: string
}

export interface CreateRenewalPayload {
  extendDays: number
}
