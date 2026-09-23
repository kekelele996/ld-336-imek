import { HttpClient } from '@angular/common/http';
import { Observable, map } from 'rxjs';
import { ApiResp, MaintenancePolicy } from '../models';
import { API_BASE, extractData } from '../utils/request';

export function maintenancePolicyListApi(http: HttpClient): Observable<MaintenancePolicy[]> {
  return http.get<ApiResp<MaintenancePolicy[]>>(`${API_BASE}/v1/maintenance-policies`).pipe(map(extractData));
}

export function maintenancePolicyUpdateApi(http: HttpClient, items: MaintenancePolicy[]): Observable<MaintenancePolicy[]> {
  return http.put<ApiResp<MaintenancePolicy[]>>(`${API_BASE}/v1/maintenance-policies`, { items }).pipe(map(extractData));
}
