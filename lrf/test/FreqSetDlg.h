// FreqSetDlg.h : interface of the CFreqSetDlg class
//
/////////////////////////////////////////////////////////////////////////////
#pragma once


class CFreqSetDlg : public CDialogImpl<CFreqSetDlg>,
    public CWinDataExchange<CFreqSetDlg>
{
public:
	CFreqSetDlg();
	enum { IDD = IDD_DLG_SETFREQ };


	BEGIN_MSG_MAP(CFreqSetDlg)
		MESSAGE_HANDLER(WM_INITDIALOG, OnInitDialog)
		COMMAND_ID_HANDLER(IDOK, OnOK)
		COMMAND_ID_HANDLER(IDCANCEL, OnCancel)
        MSG_WM_HSCROLL(OnHScroll)
    END_MSG_MAP()


	BEGIN_DDX_MAP(CFreqSetDlg)
        DDX_CONTROL_HANDLE(IDC_SLIDER_FREQ, m_sld_freq)
        DDX_CONTROL_HANDLE(IDC_EDIT_FREQ, m_edt_freq)
	END_DDX_MAP();
	
	LRESULT OnInitDialog(UINT /*uMsg*/, WPARAM /*wParam*/, LPARAM /*lParam*/, BOOL& /*bHandled*/);
	LRESULT OnOK(WORD /*wNotifyCode*/, WORD wID, HWND /*hWndCtl*/, BOOL& /*bHandled*/);
	LRESULT OnCancel(WORD /*wNotifyCode*/, WORD wID, HWND /*hWndCtl*/, BOOL& /*bHandled*/);
    void OnHScroll(UINT nSBCode, LPARAM /*lParam*/, CScrollBar pScrollBar);

    CTrackBarCtrl   m_sld_freq;
    CEdit           m_edt_freq;
protected:


public:
    LRESULT OnBnClickedOk(WORD /*wNotifyCode*/, WORD /*wID*/, HWND /*hWndCtl*/, BOOL& /*bHandled*/);
};

/////////////////////////////////////////////////////////////////////////////

//{{AFX_INSERT_LOCATION}}
// VisualFC AppWizard will insert additional declarations immediately before the previous line.
