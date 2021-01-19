// FreqSetDlg.cpp : implementation of the CFreqSetDlg class
//
/////////////////////////////////////////////////////////////////////////////
#include "stdafx.h"
#include "resource.h"
#include "drvlogic\lidarmgr.h"
#include "FreqSetDlg.h"

CFreqSetDlg::CFreqSetDlg()
{
}

LRESULT CFreqSetDlg::OnInitDialog(UINT /*uMsg*/, WPARAM /*wParam*/, LPARAM /*lParam*/, BOOL& /*bHandled*/)
{
	CenterWindow(GetParent());
    this->DoDataExchange();

    m_sld_freq.SetRange(0,MAX_MOTOR_PWM);
    m_sld_freq.SetTicFreq(1);
    m_sld_freq.SetPos(DEFAULT_MOTOR_PWM);
    CString str;
    str.Format("%d", DEFAULT_MOTOR_PWM);
    m_edt_freq.SetWindowTextA(str);

	return TRUE;
}

LRESULT CFreqSetDlg::OnOK(WORD /*wNotifyCode*/, WORD wID, HWND /*hWndCtl*/, BOOL& /*bHandled*/)
{
    char data[10];
    m_edt_freq.GetWindowTextA(data,_countof(data));
    _u16 pwm = atoi(data);

    if (pwm >= MAX_MOTOR_PWM) {
        pwm = MAX_MOTOR_PWM;
        CString str;
        str.Format("%d", pwm);
        m_edt_freq.SetWindowTextA(str);
    }

    m_sld_freq.SetPos(pwm);
    LidarMgr::GetInstance().lidar_drv->setMotorPWM(pwm);
	return 0;
}

LRESULT CFreqSetDlg::OnCancel(WORD /*wNotifyCode*/, WORD wID, HWND /*hWndCtl*/, BOOL& /*bHandled*/)
{
	EndDialog(wID);
	return 0;
}

void CFreqSetDlg::OnHScroll(UINT nSBCode, LPARAM /*lParam*/, CScrollBar pScrollBar)
{
    if (pScrollBar.m_hWnd == m_sld_freq.m_hWnd)
    {
        int realPos = m_sld_freq.GetPos();

        CString str;
        str.Format("%d", realPos);
        m_edt_freq.SetWindowTextA(str);
        LidarMgr::GetInstance().lidar_drv->setMotorPWM(realPos);
    }
}
