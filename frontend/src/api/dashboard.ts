import { get } from '../utils/request'
import { API_PATHS } from '../constants/apiPaths'

export interface OverdueBorrow {
  id: number
  equipmentId: number
  equipmentName: string
  equipmentCode: string
  borrowerId: number
  borrowerName: string
  expectedReturnDate: string
  overdueDays: number
}

export interface DashboardStats {
  statusDistribution: Record<string, number>
  borrowStatusCounts: Record<string, number>
  topBorrows: { equipmentId: number; name: string; code: string; count: number }[]
  expiringWarranty: { id: number; name: string; code: string; warrantyExpiry?: string }[]
  overdueBorrows: OverdueBorrow[]
  pendingBorrows: number
  pendingRenewals: number
  pendingReservations: number
  overdueCount: number
}

export function fetchDashboardStats() {
  return get<DashboardStats>(API_PATHS.dashboardStats)
}
