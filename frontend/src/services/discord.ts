import { api, type ApiResponse } from '@/lib/api';
import type { DiscordDeliveryHistory, DiscordIntegration, DiscordIntegrationInput } from '@/types/discord';

class DiscordService {
  list(): Promise<ApiResponse<DiscordIntegration[]>> { return api.get('/integrations/discord'); }
  create(input: DiscordIntegrationInput): Promise<ApiResponse<DiscordIntegration>> { return api.post('/integrations/discord', input); }
  update(id: string, input: DiscordIntegrationInput): Promise<ApiResponse<DiscordIntegration>> { return api.put(`/integrations/discord/${id}`, input); }
  remove(id: string): Promise<ApiResponse<null>> { return api.delete(`/integrations/discord/${id}`); }
  test(id: string): Promise<ApiResponse<{ success: boolean }>> { return api.post(`/integrations/discord/${id}/test`); }
  history(id: string): Promise<ApiResponse<{ data: DiscordDeliveryHistory[]; total: number }>> { return api.get(`/integrations/discord/${id}/history`); }
}
export const discordService = new DiscordService();
