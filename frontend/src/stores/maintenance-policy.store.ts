import { Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { MaintenancePolicy } from '../models';
import { maintenancePolicyListApi, maintenancePolicyUpdateApi } from '../api/maintenance-policy.api';

@Injectable({ providedIn: 'root' })
export class MaintenancePolicyStore {
  readonly list = signal<MaintenancePolicy[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);

  constructor(private http: HttpClient) {}

  load(): void {
    this.loading.set(true);
    maintenancePolicyListApi(this.http).subscribe({
      next: (list) => {
        this.list.set(list);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  save(items: MaintenancePolicy[], onDone: (ok: boolean) => void): void {
    this.saving.set(true);
    maintenancePolicyUpdateApi(this.http, items).subscribe({
      next: (list) => {
        this.list.set(list);
        this.saving.set(false);
        onDone(true);
      },
      error: () => {
        this.saving.set(false);
        onDone(false);
      },
    });
  }
}
