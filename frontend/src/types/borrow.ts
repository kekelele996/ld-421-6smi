export interface PendingRenewalBrief {
  id: number
  extendDays: number
  requestedDueDate: string
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
  originalExpectedReturn: string
  actualReturnDate?: string
  reason: string
  status: string
  approverId?: number
  approverName?: string
  returnCondition?: string
  pendingRenewal?: PendingRenewalBrief
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

export interface RenewalRecord {
  id: number
  borrowId: number
  equipmentId?: number
  equipmentName?: string
  applicantId: number
  applicantName?: string
  extendDays: number
  currentDueDate: string
  requestedDueDate: string
  reason: string
  status: string
  reviewerId?: number
  reviewerName?: string
  reviewComment?: string
  reviewedAt?: string
  createdAt: string
}

export interface CreateRenewalPayload {
  extendDays: number
  reason?: string
}
