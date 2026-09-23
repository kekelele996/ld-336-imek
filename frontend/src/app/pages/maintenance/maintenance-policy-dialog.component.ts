import { Component, Inject, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormArray, FormBuilder, ReactiveFormsModule } from '@angular/forms';
import { MatDialogModule, MatDialogRef, MAT_DIALOG_DATA } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatButtonModule } from '@angular/material/button';
import { MatTableModule } from '@angular/material/table';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MaintenancePolicy } from '../../../models';
import { MaintenancePolicyStore } from '../../../stores/maintenance-policy.store';
import { POLICY_TYPE_COLUMNS } from './policy-columns';

export interface PolicyDialogData {
  saving: boolean;
}

@Component({
  selector: 'app-maintenance-policy-dialog',
  standalone: true,
  imports: [
    CommonModule, ReactiveFormsModule, MatDialogModule, MatFormFieldModule, MatInputModule,
    MatButtonModule, MatTableModule, MatSlideToggleModule, MatProgressSpinnerModule,
  ],
  template: `
    <h2 mat-dialog-title>保养周期策略（按设备类别）</h2>
    <mat-dialog-content>
      <p class="hint">周期单位为天，关闭开关即停用该类保养计划；停用后点击「生成保养计划」不会生成对应工单。</p>
      <div class="table-wrap">
        <table mat-table [dataSource]="rows.controls" class="policy-table">
          <ng-container matColumnDef="category">
            <th mat-header-cell *matHeaderCellDef>设备类别</th>
            <td mat-cell *matCellDef="let row; let i = index">{{ row.value.category }}</td>
          </ng-container>
          <ng-container *ngFor="let col of typeColumns" [matColumnDef]="col.key">
            <th mat-header-cell *matHeaderCellDef>
              {{ col.label }}
              <div class="sub">间隔(天) / 启用</div>
            </th>
            <td mat-cell *matCellDef="let row; let i = index">
              <div class="interval-cell">
                <mat-slide-toggle
                  [checked]="intervalAt(i, col.key) > 0"
                  (change)="toggleType(i, col.key, $event.checked)"
                  [aria-label]="col.label + '启用开关'">
                </mat-slide-toggle>
                <input
                  class="interval-input"
                  type="number"
                  min="1"
                  max="3650"
                  [value]="intervalAt(i, col.key) > 0 ? intervalAt(i, col.key) : col.defaultInterval"
                  [disabled]="intervalAt(i, col.key) <= 0"
                  (input)="setInterval(i, col.key, $event)"
                />
                <span class="state" [class.off]="intervalAt(i, col.key) <= 0">
                  {{ intervalAt(i, col.key) > 0 ? '每' + intervalAt(i, col.key) + '天' : '停用' }}
                </span>
              </div>
            </td>
          </ng-container>
          <tr mat-header-row *matHeaderRowDef="columns"></tr>
          <tr mat-row *matRowDef="let row; columns: columns;"></tr>
        </table>
      </div>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button (click)="close()">取消</button>
      <button mat-flat-button color="primary" [disabled]="store.saving()" (click)="save()">
        <mat-spinner *ngIf="store.saving()" diameter="18" class="inline-spinner"></mat-spinner>
        保存策略
      </button>
    </mat-dialog-actions>
  `,
  styles: [`
    .hint { color: #666; font-size: 12px; margin: 0 0 12px; }
    .table-wrap { max-height: 60vh; overflow: auto; min-width: 760px; }
    .policy-table { width: 100%; }
    .sub { font-weight: 400; font-size: 11px; color: #888; }
    .interval-cell { display: flex; align-items: center; gap: 8px; }
    .interval-input { width: 72px; padding: 4px 8px; border: 1px solid #ccc; border-radius: 4px; }
    .interval-input:disabled { background: #f5f5f5; color: #aaa; }
    .state { font-size: 12px; color: #2e7d32; white-space: nowrap; }
    .state.off { color: #d32f2f; }
    .inline-spinner { display: inline-block; margin-right: 4px; vertical-align: middle; }
  `],
})
export class MaintenancePolicyDialogComponent {
  private fb = inject(FormBuilder);
  store = inject(MaintenancePolicyStore);

  typeColumns = POLICY_TYPE_COLUMNS;
  columns = ['category', ...POLICY_TYPE_COLUMNS.map((c) => c.key)];

  rows = this.fb.nonNullable.array(
    this.store.list().map((p) => this.fb.group({
      category: [p.category],
      daily_interval: [p.daily_interval],
      weekly_interval: [p.weekly_interval],
      monthly_interval: [p.monthly_interval],
      yearly_interval: [p.yearly_interval],
    })),
  );

  constructor(
    @Inject(MAT_DIALOG_DATA) public data: PolicyDialogData,
    private dialogRef: MatDialogRef<MaintenancePolicyDialogComponent>,
  ) {}

  intervalAt(index: number, key: string): number {
    return (this.rows.at(index).get(key)?.value as number) ?? 0;
  }

  toggleType(index: number, key: string, enabled: boolean): void {
    const current = this.intervalAt(index, key);
    if (enabled) {
      const col = this.typeColumns.find((c) => c.key === key);
      this.rows.at(index).get(key)?.setValue(current > 0 ? current : (col?.defaultInterval ?? 1));
    } else {
      this.rows.at(index).get(key)?.setValue(0);
    }
  }

  setInterval(index: number, key: string, event: Event): void {
    const v = Number((event.target as HTMLInputElement).value);
    this.rows.at(index).get(key)?.setValue(Number.isFinite(v) ? v : 0);
  }

  save(): void {
    const raw = this.rows.getRawValue() as Array<Record<string, number | string>>;
    const payload: MaintenancePolicy[] = raw.map((r) => ({
      category: String(r.category),
      daily_interval: this.normalize(Number(r.daily_interval)),
      weekly_interval: this.normalize(Number(r.weekly_interval)),
      monthly_interval: this.normalize(Number(r.monthly_interval)),
      yearly_interval: this.normalize(Number(r.yearly_interval)),
    }));
    this.store.save(payload, (ok) => {
      if (ok) this.dialogRef.close(true);
    });
  }

  private normalize(v: number): number {
    if (!Number.isFinite(v) || v <= 0) return 0;
    return Math.min(3650, Math.round(v));
  }

  close(): void {
    this.dialogRef.close(null);
  }
}
