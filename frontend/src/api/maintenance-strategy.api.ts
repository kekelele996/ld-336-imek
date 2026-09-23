import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable, map } from 'rxjs';
import { ApiResp, MaintenanceStrategy } from '../models';
import { API_BASE, extractData } from '../utils/request';

export interface CreateStrategyPayload {
  category: string;
  type: string;
  interval_days: number;
  enabled?: boolean;
  remark?: string;
}

export interface UpdateStrategyPayload {
  interval_days?: number;
  enabled?: boolean;
  remark?: string;
}

export function strategyListApi(http: HttpClient, category?: string, enabled?: boolean): Observable<MaintenanceStrategy[]> {
  let params = new HttpParams();
  if (category) params = params.set('category', category);
  if (enabled !== undefined) params = params.set('enabled', enabled);
  return http.get<ApiResp<MaintenanceStrategy[]>>(`${API_BASE}/v1/maintenance-strategies`, { params }).pipe(map(extractData));
}

export function strategyCreateApi(http: HttpClient, payload: CreateStrategyPayload): Observable<MaintenanceStrategy> {
  return http.post<ApiResp<MaintenanceStrategy>>(`${API_BASE}/v1/maintenance-strategies`, payload).pipe(map(extractData));
}

export function strategyUpdateApi(http: HttpClient, id: number, payload: UpdateStrategyPayload): Observable<MaintenanceStrategy> {
  return http.put<ApiResp<MaintenanceStrategy>>(`${API_BASE}/v1/maintenance-strategies/${id}`, payload).pipe(map(extractData));
}
