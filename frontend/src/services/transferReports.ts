import { api, type ApiResponse } from '@/lib/api';
import type { LatestTransferReports, TransferReportSettings } from '@/types/transferReports';

class TransferReportsService {
  getSettings(): Promise<ApiResponse<TransferReportSettings>> { return api.get('/reports/transfer/settings'); }
  updateSettings(settings: TransferReportSettings): Promise<ApiResponse<TransferReportSettings>> { return api.put('/reports/transfer/settings', settings); }
  getLatest(): Promise<ApiResponse<LatestTransferReports>> { return api.get('/reports/transfer/latest'); }
}
export const transferReportsService = new TransferReportsService();
